package rtmp

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"net"
	"net/url"
	"time"

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

type RTMPClient struct {
	url         *url.URL
	client      *gortmplib.Client
	signal      chan any
	packetQueue chan *av.Packet
}

func New(parsedUrl *url.URL) *RTMPClient {
	return &RTMPClient{
		url:         parsedUrl,
		signal:      make(chan any, 1),
		packetQueue: make(chan *av.Packet),
	}
}

func (r *RTMPClient) Dial() error {
	if _, _, err := net.SplitHostPort(r.url.Host); err != nil {
		if r.url.Scheme == "rtmps" {
			r.url.Host += DefaultRtmpsPort
		} else {
			r.url.Host += DefaultRtmpPort
		}
	}

	r.client = &gortmplib.Client{
		URL: r.url, Publish: false,
		TLSConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	return r.client.Initialize(context.Background())
}

func (r *RTMPClient) Close() {
	if r.client != nil {
		r.client.Close()
	}
}

func (r *RTMPClient) onDataH26x(index int8, pts, dts time.Duration, au [][]byte) {
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
		r.packetQueue <- &av.Packet{
			Idx:             index,
			IsKeyFrame:      isKeyFrame,
			CompositionTime: pts - dts,
			Time:            dts,
			Data:            buf,
		}
	}
}

func (r *RTMPClient) CodecData() ([]av.CodecData, error) {
	reader := &gortmplib.Reader{Conn: r.client}
	if err := reader.Initialize(); err != nil {
		return nil, err
	}

	var codecs []av.CodecData
	for index, track := range reader.Tracks() {
		switch track := track.(type) {
		case *format.H264:
			h264Codec, err := h264parser.NewCodecDataFromSPSAndPPS(track.SPS, track.PPS)
			if err != nil {
				return nil, err
			}
			codecs = append(codecs, h264Codec)
			reader.OnDataH264(track, func(pts, dts time.Duration, au [][]byte) { r.onDataH26x(int8(index), pts, dts, au) })
		case *format.H265:
			h265Codec, err := h265parser.NewCodecDataFromVPSAndSPSAndPPS(track.VPS, track.SPS, track.PPS)
			if err != nil {
				return nil, err
			}
			codecs = append(codecs, h265Codec)
			reader.OnDataH265(track, func(pts, dts time.Duration, au [][]byte) { r.onDataH26x(int8(index), pts, dts, au) })
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
			reader.OnDataMPEG4Audio(track, func(pts time.Duration, au []byte) { r.packetQueue <- &av.Packet{Idx: int8(index), Time: pts, Data: au} })
		default:
			return nil, fmt.Errorf("unsupported codec: %T", track)
		}
	}

	go func() {
		for {
			_ = r.client.NetConn().SetReadDeadline(time.Now().Add(30 * time.Second))
			if err := reader.Read(); err != nil {
				r.signal <- err
				return
			}
		}
	}()

	return codecs, nil
}

func (r *RTMPClient) PacketQueue() <-chan *av.Packet {
	return r.packetQueue
}

func (r *RTMPClient) CloseCh() <-chan any {
	return r.signal
}
