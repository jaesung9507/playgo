package hls

import (
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/bluenviron/gohlslib/v2"
	"github.com/bluenviron/gohlslib/v2/pkg/codecs"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/h264"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/mpeg4audio"
	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/codec/aacparser"
	"github.com/deepch/vdk/codec/h264parser"
)

type Client struct {
	url         *url.URL
	client      *gohlslib.Client
	signal      chan any
	packetQueue chan *av.Packet
	h264Codec   *codecs.H264
	aacCodec    *codecs.MPEG4Audio
}

func New(parsedUrl *url.URL) *Client {
	return &Client{
		url:         parsedUrl,
		signal:      make(chan any, 1),
		packetQueue: make(chan *av.Packet),
	}
}

func (c *Client) Dial() error {
	c.client = &gohlslib.Client{
		URI: c.url.String(),
		HTTPClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		},
		OnRequest: func(r *http.Request) {
			if r.URL.RawQuery == "" && c.url.RawQuery != "" {
				r.URL.RawQuery = c.url.RawQuery
			}
		},
	}

	log.Printf("[HLS] dial: %s", c.url.String())
	c.client.OnTracks = func(tracks []*gohlslib.Track) error {
		for i, track := range tracks {
			log.Printf("[HLS] on track %d: %T", i, track.Codec)
			switch codec := track.Codec.(type) {
			case *codecs.H264:
				c.h264Codec = codec
				buf := bytes.NewBuffer(nil)
				c.client.OnDataH26x(track, func(pts, dts int64, au [][]byte) {
					buf.Reset()
					var isKeyFrame bool
					for _, nalu := range au {
						switch h264.NALUType(nalu[0] & 0x1F) {
						case h264.NALUTypeSPS:
							if c.h264Codec.SPS == nil {
								c.h264Codec.SPS = nalu
								if c.h264Codec.PPS != nil {
									c.signal <- true
								}
							}
						case h264.NALUTypePPS:
							if c.h264Codec.PPS == nil {
								c.h264Codec.PPS = nalu
								if c.h264Codec.SPS != nil {
									c.signal <- true
								}
							}
						case h264.NALUTypeIDR:
							isKeyFrame = true
							fallthrough
						case h264.NALUTypeNonIDR, h264.NALUTypeDataPartitionA, h264.NALUTypeDataPartitionB, h264.NALUTypeDataPartitionC:
							binary.Write(buf, binary.BigEndian, uint32(len(nalu)))
							buf.Write(nalu)
						}
					}

					if bufLen := buf.Len(); bufLen > 0 {
						data := make([]byte, bufLen)
						copy(data, buf.Bytes())
						pts := time.Duration(pts) * time.Second / time.Duration(track.ClockRate)
						dts := time.Duration(dts) * time.Second / time.Duration(track.ClockRate)
						c.packetQueue <- &av.Packet{
							Idx:             0,
							IsKeyFrame:      isKeyFrame,
							CompositionTime: pts - dts,
							Time:            dts,
							Data:            data,
						}
					}
				})
			case *codecs.MPEG4Audio:
				c.aacCodec = codec
				c.client.OnDataMPEG4Audio(track, func(pts int64, aus [][]byte) {
					for j, au := range aus {
						delta := time.Duration(j) * mpeg4audio.SamplesPerAccessUnit * time.Second / time.Duration(codec.Config.SampleRate)
						c.packetQueue <- &av.Packet{
							Idx:  1,
							Time: (time.Duration(pts) * time.Second / time.Duration(track.ClockRate)) + delta,
							Data: au,
						}
					}
				})
			default:
				c.signal <- fmt.Errorf("unsupported codec: %T", track.Codec)
			}
		}

		if c.h264Codec != nil && c.h264Codec.SPS != nil && c.h264Codec.PPS != nil {
			c.signal <- true
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

	signal := <-c.signal
	if err, ok := signal.(error); ok {
		return nil, err
	}

	var codecs []av.CodecData
	if c.h264Codec != nil && c.h264Codec.SPS != nil && c.h264Codec.PPS != nil {
		h264Codec, err := h264parser.NewCodecDataFromSPSAndPPS(c.h264Codec.SPS, c.h264Codec.PPS)
		if err != nil {
			return nil, err
		}
		codecs = append(codecs, h264Codec)
		log.Printf("[HLS] track %d: H264 codec ready", 0)
	}

	if c.aacCodec != nil {
		config, err := c.aacCodec.Config.Marshal()
		if err != nil {
			return nil, err
		}

		aacCodec, err := aacparser.NewCodecDataFromMPEG4AudioConfigBytes(config)
		if err != nil {
			return nil, err
		}
		codecs = append(codecs, aacCodec)
		log.Printf("[HLS] track %d: AAC codec ready", 1)
	}

	return codecs, nil
}

func (c *Client) PacketQueue() <-chan *av.Packet {
	return c.packetQueue
}

func (c *Client) CloseCh() <-chan any {
	return c.signal
}
