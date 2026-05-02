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

	"github.com/bluenviron/gohlslib/v2"
	"github.com/bluenviron/gohlslib/v2/pkg/codecs"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/h264"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/h265"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/mpeg4audio"
	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/codec/aacparser"
	"github.com/deepch/vdk/codec/h264parser"
	"github.com/deepch/vdk/codec/h265parser"
)

type Client struct {
	url         *url.URL
	client      *gohlslib.Client
	signal      chan any
	packetQueue chan *av.Packet
	tls         secure.TLS

	ready     bool
	readyCh   chan []av.CodecData
	readyOnce sync.Once
}

func New(parsedUrl *url.URL) *Client {
	return &Client{
		url:         parsedUrl,
		signal:      make(chan any, 1),
		packetQueue: make(chan *av.Packet),
		readyCh:     make(chan []av.CodecData),
	}
}

func (c *Client) readyCodec(codecs []av.CodecData) {
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
		trackCodecs := make([]av.CodecData, len(tracks))
		for i, track := range tracks {
			log.Printf("[HLS] on track %d: %T", i, track.Codec)
			switch codec := track.Codec.(type) {
			case *codecs.H264:
				buf := bytes.NewBuffer(nil)
				c.client.OnDataH26x(track, func(pts, dts int64, au [][]byte) {
					buf.Reset()
					var isKeyFrame bool
					for _, nalu := range au {
						switch h264.NALUType(nalu[0] & 0x1F) {
						case h264.NALUTypeSPS:
							codec.SPS = nalu
						case h264.NALUTypePPS:
							codec.PPS = nalu
						case h264.NALUTypeIDR:
							isKeyFrame = true
						}
						binary.Write(buf, binary.BigEndian, uint32(len(nalu)))
						buf.Write(nalu)
					}

					if trackCodecs[i] == nil && codec.SPS != nil && codec.PPS != nil {
						h264Codec, err := h264parser.NewCodecDataFromSPSAndPPS(codec.SPS, codec.PPS)
						if err != nil {
							c.signal <- err
						}

						trackCodecs[i] = h264Codec
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
						naluType := h265.NALUType((nalu[0] >> 1) & 0x3F)
						switch naluType {
						case h265.NALUType_VPS_NUT:
							codec.VPS = nalu
						case h265.NALUType_SPS_NUT:
							codec.SPS = nalu
						case h265.NALUType_PPS_NUT:
							codec.PPS = nalu
						case h265.NALUType_IDR_W_RADL, h265.NALUType_IDR_N_LP, h265.NALUType_CRA_NUT:
							isKeyFrame = true
							fallthrough
						default:
							if naluType <= 31 {
								binary.Write(buf, binary.BigEndian, uint32(len(nalu)))
								buf.Write(nalu)
							}
						}
					}

					if trackCodecs[i] == nil && codec.VPS != nil && codec.SPS != nil && codec.PPS != nil {
						h265Codec, err := h265parser.NewCodecDataFromVPSAndSPSAndPPS(codec.VPS, codec.SPS, codec.PPS)
						if err != nil {
							c.signal <- err
						}

						trackCodecs[i] = h265Codec
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
				if cfg, err := codec.Config.Marshal(); err != nil {
					c.signal <- err
				} else {
					aacCodec, err := aacparser.NewCodecDataFromMPEG4AudioConfigBytes(cfg)
					if err != nil {
						c.signal <- err
					}

					trackCodecs[i] = aacCodec
					log.Printf("[HLS] track %d: AAC codec ready", i)
					c.readyCodec(trackCodecs)
				}

				c.client.OnDataMPEG4Audio(track, func(pts int64, aus [][]byte) {
					if c.ready {
						for j, au := range aus {
							delta := time.Duration(j) * mpeg4audio.SamplesPerAccessUnit * time.Second / time.Duration(codec.Config.SampleRate)
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

func (c *Client) CodecData() ([]av.CodecData, error) {
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
