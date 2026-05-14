package hls

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"slices"
	"sync"
	"time"

	"github.com/jaesung9507/playgo/secure"
	"github.com/jaesung9507/playgo/stream/codec"
	"github.com/jaesung9507/playgo/stream/codec/aac"
	"github.com/jaesung9507/playgo/stream/codec/h26x/h264"
	"github.com/jaesung9507/playgo/stream/codec/h26x/h265"

	"github.com/bluenviron/gohlslib/v2"
	"github.com/bluenviron/gohlslib/v2/pkg/codecs"
	"github.com/deepch/vdk/av"
)

type Client struct {
	url         *url.URL
	client      *gohlslib.Client
	signal      chan any
	packetQueue chan *av.Packet
	tls         secure.TLS

	ready     bool
	readyCh   chan []codec.Codec
	readyOnce sync.Once
}

func New(parsedUrl *url.URL) *Client {
	return &Client{
		url:         parsedUrl,
		signal:      make(chan any, 1),
		packetQueue: make(chan *av.Packet),
		readyCh:     make(chan []codec.Codec),
	}
}

func (c *Client) readyCodec(codecs []codec.Codec) {
	for _, codec := range codecs {
		if codec == nil {
			return
		}
	}

	c.readyOnce.Do(func() {
		c.readyCh <- codecs
		close(c.readyCh)
		c.ready = true
	})
}

func (c *Client) DialWithHeader(header map[string]string) error {
	return c.dial(header)
}

func (c *Client) Dial() error {
	return c.dial(nil)
}

func (c *Client) dial(header map[string]string) error {
	c.client = &gohlslib.Client{
		URI: c.url.String(),
		HTTPClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: c.tls.Config(),
			},
		},
		OnRequest: func(r *http.Request) {
			if r.URL.RawQuery == "" && c.url.RawQuery != "" {
				r.URL.RawQuery = c.url.RawQuery
			}

			for k, v := range header {
				r.Header.Set(k, v)
			}
		},
	}

	log.Printf("[HLS] dial: %s", c.url.String())
	c.client.OnTracks = func(tracks []*gohlslib.Track) error {
		trackCodecs := make([]codec.Codec, len(tracks))
		for i, track := range tracks {
			log.Printf("[HLS] on track %d: %T", i, track.Codec)
			switch codec := track.Codec.(type) {
			case *codecs.H264:
				buf := bytes.NewBuffer(nil)
				c.client.OnDataH26x(track, func(pts, dts int64, au [][]byte) {
					buf.Reset()
					var isKeyFrame bool
					for _, nalu := range au {
						switch h264.ParseNALUType(nalu[0]) {
						case h264.NALUnitSPS:
							codec.SPS = nalu
						case h264.NALUnitPPS:
							codec.PPS = nalu
						case h264.NALUnitIDRSlice:
							isKeyFrame = true
						}
						binary.Write(buf, binary.BigEndian, uint32(len(nalu)))
						buf.Write(nalu)
					}

					if trackCodecs[i] == nil && codec.SPS != nil && codec.PPS != nil {
						trackCodecs[i] = &h264.Codec{SPS: codec.SPS, PPS: codec.PPS}
						log.Printf("[HLS] track %d: H264 codec ready", i)
						c.readyCodec(trackCodecs)
					}

					if c.ready && buf.Len() > 0 {
						pts := time.Duration(pts) * time.Second / time.Duration(track.ClockRate)
						dts := time.Duration(dts) * time.Second / time.Duration(track.ClockRate)
						c.packetQueue <- &av.Packet{
							Idx:             int8(i),
							IsKeyFrame:      isKeyFrame,
							CompositionTime: pts - dts,
							Time:            dts,
							Data:            slices.Clone(buf.Bytes()),
						}
					}
				})
			case *codecs.H265:
				buf := bytes.NewBuffer(nil)
				c.client.OnDataH26x(track, func(pts, dts int64, au [][]byte) {
					buf.Reset()
					var isKeyFrame bool
					for _, nalu := range au {
						naluType := h265.ParseNALUType(nalu[0])
						switch naluType {
						case h265.NALUnitVPS:
							codec.VPS = nalu
						case h265.NALUnitSPS:
							codec.SPS = nalu
						case h265.NALUnitPPS:
							codec.PPS = nalu
						case h265.NALUnitIDRWRADL, h265.NALUnitIDRNLP, h265.NALUnitCRANUT:
							isKeyFrame = true
							fallthrough
						default:
							if naluType <= h265.NALUnitRSVVCL31 {
								binary.Write(buf, binary.BigEndian, uint32(len(nalu)))
								buf.Write(nalu)
							}
						}
					}

					if trackCodecs[i] == nil && codec.VPS != nil && codec.SPS != nil && codec.PPS != nil {
						trackCodecs[i] = &h265.Codec{VPS: codec.VPS, SPS: codec.SPS, PPS: codec.PPS}
						log.Printf("[HLS] track %d: H265 codec ready", i)
						c.readyCodec(trackCodecs)
					}

					if c.ready && buf.Len() > 0 {
						pts := time.Duration(pts) * time.Second / time.Duration(track.ClockRate)
						dts := time.Duration(dts) * time.Second / time.Duration(track.ClockRate)
						c.packetQueue <- &av.Packet{
							Idx:             int8(i),
							IsKeyFrame:      isKeyFrame,
							CompositionTime: pts - dts,
							Time:            dts,
							Data:            slices.Clone(buf.Bytes()),
						}
					}
				})
			case *codecs.MPEG4Audio:
				if asc, err := codec.Config.Marshal(); err != nil {
					c.signal <- err
				} else {
					trackCodecs[i] = &aac.Codec{ASC: asc, Config: codec.Config}
					log.Printf("[HLS] track %d: AAC codec ready", i)
					c.readyCodec(trackCodecs)
				}

				c.client.OnDataMPEG4Audio(track, func(pts int64, aus [][]byte) {
					if c.ready {
						for j, au := range aus {
							delta := time.Duration(j) * aac.SamplesPerAccessUnit * time.Second / time.Duration(codec.Config.SampleRate)
							c.packetQueue <- &av.Packet{
								Idx:  int8(i),
								Time: (time.Duration(pts) * time.Second / time.Duration(track.ClockRate)) + delta,
								Data: au,
							}
						}
					}
				})
			default:
				c.signal <- fmt.Errorf("unsupported codec: %T", track.Codec)
			}
		}

		return nil
	}

	return c.client.Start()
}

func (c *Client) Close() {
	log.Print("[HLS] close")
	if c.client != nil {
		c.client.Close()
	}
}

func (c *Client) CodecData() ([]codec.Codec, error) {
	go func() {
		c.signal <- c.client.Wait2()
	}()

	select {
	case codecs := <-c.readyCh:
		return codecs, nil
	case err := <-c.signal:
		return nil, err.(error)
	}
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
