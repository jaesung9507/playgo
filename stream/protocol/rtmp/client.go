package rtmp

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"net/url"
	"time"

	"github.com/jaesung9507/playgo/secure"

	"github.com/bluenviron/gortmplib"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/h264"
	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/codec/aacparser"
	"github.com/deepch/vdk/codec/h264parser"
	"github.com/deepch/vdk/codec/h265parser"
)

const (
	DefaultRtmpPort  = ":1935"
	DefaultRtmpsPort = ":443"
)

type Client struct {
	url         *url.URL
	client      *gortmplib.Client
	signal      chan any
	packetQueue chan *av.Packet
	tls         secure.TLS
}

func New(parsedUrl *url.URL) *Client {
	return &Client{
		url:         parsedUrl,
		signal:      make(chan any, 1),
		packetQueue: make(chan *av.Packet),
	}
}

func (c *Client) Dial() error {
	log.Printf("[RTMP] dial: %s", c.url.String())
	if _, _, err := net.SplitHostPort(c.url.Host); err != nil {
		if c.url.Scheme == "rtmps" {
			c.url.Host += DefaultRtmpsPort
		} else {
			c.url.Host += DefaultRtmpPort
		}
	}

	client := &gortmplib.Client{
		URL:       c.url,
		Publish:   false,
		TLSConfig: c.tls.Config(),
	}

	if err := client.Initialize(context.Background()); err != nil {
		return err
	}
	c.client = client

	return nil
}

func (c *Client) Close() {
	log.Print("[RTMP] close")
	if c.client != nil {
		c.client.Close()
	}
}

func (c *Client) onDataH26x(index int8, pts, dts time.Duration, au [][]byte) {
	var isKeyFrame bool
	buf := bytes.NewBuffer(nil)
	for _, nalu := range au {
		switch h264.NALUType(nalu[0] & 0x1F) {
		case h264.NALUTypeSPS, h264.NALUTypePPS:
		case h264.NALUTypeIDR:
			isKeyFrame = true
			fallthrough
		default:
			b := make([]byte, 4+len(nalu))
			binary.BigEndian.PutUint32(b, uint32(len(nalu)))
			copy(b[4:], nalu)
			buf.Write(b)
		}
	}

	if buf := buf.Bytes(); len(buf) > 0 {
		c.packetQueue <- &av.Packet{
			Idx:             index,
			IsKeyFrame:      isKeyFrame,
			CompositionTime: pts - dts,
			Time:            dts,
			Data:            buf,
		}
	}
}

func (c *Client) CodecData() ([]av.CodecData, error) {
	reader := &gortmplib.Reader{Conn: c.client}
	if err := reader.Initialize(); err != nil {
		return nil, err
	}

	var codecs []av.CodecData
	for index, track := range reader.Tracks() {
		log.Printf("[RTMP] on track %d: %T", index, track)
		switch track := track.(type) {
		case *format.H264:
			h264Codec, err := h264parser.NewCodecDataFromSPSAndPPS(track.SPS, track.PPS)
			if err != nil {
				return nil, err
			}
			codecs = append(codecs, h264Codec)
			log.Printf("[RTMP] track %d: H264 codec ready", index)
			reader.OnDataH264(track, func(pts, dts time.Duration, au [][]byte) { c.onDataH26x(int8(index), pts, dts, au) })
		case *format.H265:
			h265Codec, err := h265parser.NewCodecDataFromVPSAndSPSAndPPS(track.VPS, track.SPS, track.PPS)
			if err != nil {
				return nil, err
			}
			codecs = append(codecs, h265Codec)
			log.Printf("[RTMP] track %d: H265 codec ready", index)
			reader.OnDataH265(track, func(pts, dts time.Duration, au [][]byte) { c.onDataH26x(int8(index), pts, dts, au) })
		case *format.MPEG4Audio:
			config, err := track.Config.Marshal()
			if err != nil {
				return nil, err
			}
			aacCodec, err := aacparser.NewCodecDataFromMPEG4AudioConfigBytes(config)
			if err != nil {
				return nil, err
			}
			codecs = append(codecs, aacCodec)
			log.Printf("[RTMP] track %d: AAC codec ready", index)
			reader.OnDataMPEG4Audio(track, func(pts time.Duration, au []byte) { c.packetQueue <- &av.Packet{Idx: int8(index), Time: pts, Data: au} })
		default:
			return nil, fmt.Errorf("unsupported codec: %T", track)
		}
	}

	go func() {
		for {
			_ = c.client.NetConn().SetReadDeadline(time.Now().Add(30 * time.Second))
			if err := reader.Read(); err != nil {
				c.signal <- err
				return
			}
		}
	}()

	return codecs, nil
}

func (c *Client) PacketQueue() <-chan *av.Packet {
	return c.packetQueue
}

func (c *Client) CloseCh() <-chan any {
	return c.signal
}

func (c *Client) Secure() (bool, bool, map[string]string) {
	return c.tls.Info()
}
