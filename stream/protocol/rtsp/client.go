package rtsp

import (
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"net/url"
	"slices"
	"time"

	"github.com/bluenviron/gortsplib/v5"
	"github.com/bluenviron/gortsplib/v5/pkg/base"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/h264"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/mpeg4audio"
	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/codec/aacparser"
	"github.com/deepch/vdk/codec/h264parser"
	"github.com/pion/rtp"
)

const (
	DefaultRtspPort  = ":554"
	DefaultRtspsPort = ":322"
)

type Client struct {
	url         *url.URL
	client      *gortsplib.Client
	signal      chan any
	packetQueue chan *av.Packet
}

func New(parsedUrl *url.URL) *Client {
	return &Client{
		url:         parsedUrl,
		signal:      make(chan any, 1),
		packetQueue: make(chan *av.Packet, 128),
	}
}

func (c *Client) Dial() error {
	log.Printf("[RTSP] dial: %s", c.url.String())
	u, err := base.ParseURL(c.url.String())
	if err != nil {
		return err
	}
	c.url = (*url.URL)(u)

	host := c.url.Host
	if _, _, err := net.SplitHostPort(host); err != nil {
		if c.url.Scheme == "rtsps" {
			host += DefaultRtspsPort
		} else {
			host += DefaultRtspPort
		}
	}

	c.client = &gortsplib.Client{
		Scheme: u.Scheme,
		Host:   host,
		TLSConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	return c.client.Start()
}

func (c *Client) Close() {
	log.Print("[RTSP] close")
	if c.client != nil {
		c.client.Close()
	}
}

func (c *Client) CodecData() ([]av.CodecData, error) {
	desc, _, err := c.client.Describe((*base.URL)(c.url))
	if err != nil {
		return nil, err
	}

	trackCodecs := make([]av.CodecData, len(desc.Medias))
	for i, media := range desc.Medias {
		if _, err = c.client.Setup(desc.BaseURL, media, 0, 0); err != nil {
			return nil, err
		}

		for _, f := range media.Formats {
			switch f := f.(type) {
			case *format.H264:
				h264Codec, err := h264parser.NewCodecDataFromSPSAndPPS(f.SPS, f.PPS)
				if err != nil {
					return nil, err
				}
				trackCodecs[i] = h264Codec
				log.Printf("[RTSP] track %d: H264 codec ready", i)

				dec, err := f.CreateDecoder()
				if err != nil {
					return nil, err
				}

				dtsExtractor := &h264.DTSExtractor{}
				dtsExtractor.Initialize()
				dtsExtractor.Extract([][]byte{f.SPS, f.PPS}, 0)

				buf := bytes.NewBuffer(nil)
				c.client.OnPacketRTP(media, f, func(pkt *rtp.Packet) {
					pts, ok := c.client.PacketPTS(media, pkt)
					if !ok {
						return
					}

					au, err := dec.Decode(pkt)
					if err != nil {
						return
					}

					dts, err := dtsExtractor.Extract(au, pts)
					if err != nil {
						dts = pts
					}

					buf.Reset()
					var isKeyFrame bool
					for _, nalu := range au {
						switch h264.NALUType(nalu[0] & 0x1F) {
						case h264.NALUTypeSPS:
							f.SPS = nalu
						case h264.NALUTypePPS:
							f.PPS = nalu
						case h264.NALUTypeIDR:
							isKeyFrame = true
						}
						binary.Write(buf, binary.BigEndian, uint32(len(nalu)))
						buf.Write(nalu)
					}

					if buf.Len() > 0 {
						clockRate := time.Duration(f.ClockRate())
						pts := time.Duration(pts) * time.Second / time.Duration(clockRate)
						dts := time.Duration(dts) * time.Second / time.Duration(clockRate)

						c.packetQueue <- &av.Packet{
							Idx:             int8(i),
							IsKeyFrame:      isKeyFrame,
							CompositionTime: pts - dts,
							Time:            pts,
							Data:            slices.Clone(buf.Bytes()),
						}
					}
				})
			case *format.MPEG4Audio:
				cfg, err := f.Config.Marshal()
				if err != nil {
					return nil, err
				}

				aacCodec, err := aacparser.NewCodecDataFromMPEG4AudioConfigBytes(cfg)
				if err != nil {
					return nil, err
				}
				trackCodecs[i] = aacCodec
				log.Printf("[RTSP] track %d: AAC codec ready", i)

				dec, err := f.CreateDecoder()
				if err != nil {
					return nil, err
				}

				c.client.OnPacketRTP(media, f, func(pkt *rtp.Packet) {
					pts, ok := c.client.PacketPTS(media, pkt)
					if !ok {
						return
					}

					aus, err := dec.Decode(pkt)
					if err != nil {
						return
					}

					clockRate := f.ClockRate()
					for j, au := range aus {
						delta := time.Duration(j) * mpeg4audio.SamplesPerAccessUnit * time.Second / time.Duration(clockRate)
						c.packetQueue <- &av.Packet{
							Idx:  int8(i),
							Time: (time.Duration(pts) * time.Second / time.Duration(clockRate)) + delta,
							Data: au,
						}
					}
				})
			default:
				return nil, fmt.Errorf("unsupported codec: %T", f)
			}
		}
	}

	if _, err = c.client.Play(nil); err != nil {
		return nil, err
	}

	go func() {
		c.signal <- c.client.Wait()
	}()

	return trackCodecs, nil
}

func (c *Client) PacketQueue() <-chan *av.Packet {
	return c.packetQueue
}

func (c *Client) CloseCh() <-chan any {
	return c.signal
}
