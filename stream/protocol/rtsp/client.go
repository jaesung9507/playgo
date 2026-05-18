package rtsp

import (
	"fmt"
	"log"
	"net"
	"net/url"
	"time"

	"github.com/jaesung9507/playgo/secure"
	"github.com/jaesung9507/playgo/stream"
	"github.com/jaesung9507/playgo/stream/codec/aac"
	"github.com/jaesung9507/playgo/stream/codec/h26x/h264"

	"github.com/bluenviron/gortsplib/v5"
	"github.com/bluenviron/gortsplib/v5/pkg/base"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
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
	packetQueue chan *stream.Packet
	tls         secure.TLS
}

func New(parsedUrl *url.URL) *Client {
	return &Client{
		url:         parsedUrl,
		signal:      make(chan any, 1),
		packetQueue: make(chan *stream.Packet, 128),
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
		Scheme:    u.Scheme,
		Host:      host,
		TLSConfig: c.tls.Config(),
	}

	return c.client.Start()
}

func (c *Client) Close() {
	log.Print("[RTSP] close")
	if c.client != nil {
		c.client.Close()
	}
}

func (c *Client) CodecData() ([]stream.Codec, error) {
	desc, _, err := c.client.Describe((*base.URL)(c.url))
	if err != nil {
		return nil, err
	}

	trackCodecs := make([]stream.Codec, len(desc.Medias))
	for i, media := range desc.Medias {
		if _, err = c.client.Setup(desc.BaseURL, media, 0, 0); err != nil {
			return nil, err
		}

		for _, f := range media.Formats {
			switch f := f.(type) {
			case *format.H264:
				h264Codec := &h264.Codec{SPS: f.SPS, PPS: f.PPS}
				trackCodecs[i] = h264Codec
				log.Printf("[RTSP] track %d: H264 codec ready", i)

				dec, err := f.CreateDecoder()
				if err != nil {
					return nil, err
				}

				dtsExtractor := &h264.DTSExtractor{}
				dtsExtractor.Initialize()
				dtsExtractor.Extract([][]byte{f.SPS, f.PPS}, 0)

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

					isKeyFrame, data := h264Codec.ParseAU(au)
					if len(data) > 0 {
						clockRate := time.Duration(f.ClockRate())
						pts := time.Duration(pts) * time.Second / time.Duration(clockRate)
						dts := time.Duration(dts) * time.Second / time.Duration(clockRate)

						c.packetQueue <- &stream.Packet{
							Idx:             int8(i),
							IsKeyFrame:      isKeyFrame,
							CompositionTime: pts - dts,
							Time:            pts,
							Data:            data,
						}
					}
				})
			case *format.MPEG4Audio:
				asc, err := f.Config.Marshal()
				if err != nil {
					return nil, err
				}
				trackCodecs[i] = &aac.Codec{ASC: asc, Config: *f.Config}
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
						delta := time.Duration(j) * aac.SamplesPerAccessUnit * time.Second / time.Duration(clockRate)
						c.packetQueue <- &stream.Packet{
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

func (c *Client) PacketQueue() <-chan *stream.Packet {
	return c.packetQueue
}

func (c *Client) CloseCh() <-chan any {
	return c.signal
}

func (c *Client) Secure() (bool, bool, map[string]string) {
	return c.tls.Info()
}
