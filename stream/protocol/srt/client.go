package srt

import (
	"encoding/binary"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/bluenviron/mediacommon/v2/pkg/codecs/h264"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/mpeg4audio"
	"github.com/bluenviron/mediacommon/v2/pkg/formats/mpegts"
	"github.com/bluenviron/mediacommon/v2/pkg/formats/mpegts/codecs"
	srt "github.com/datarhei/gosrt"
	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/codec/aacparser"
	"github.com/deepch/vdk/codec/h264parser"
)

type Client struct {
	url         *url.URL
	reader      *mpegts.Reader
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

func (c *Client) getConfig() (*srt.Config, error) {
	cfg := srt.DefaultConfig()
	if _, err := cfg.UnmarshalURL(c.url.String()); err != nil {
		return nil, err
	}

	if len(cfg.StreamId) <= 0 && strings.HasPrefix(c.url.Fragment, "!::") {
		cfg.StreamId = "#" + c.url.Fragment
	}

	return &cfg, nil
}

func (c *Client) Dial() error {
	cfg, err := c.getConfig()
	if err != nil {
		return err
	}

	conn, err := srt.Dial(c.url.Scheme, c.url.Host, *cfg)
	if err != nil {
		return err
	}

	c.reader = &mpegts.Reader{R: conn}
	if err = c.reader.Initialize(); err != nil {
		return err
	}

	return nil
}

func (c *Client) Close() {
	if c.reader != nil {
		if closer, ok := c.reader.R.(io.ReadCloser); ok {
			closer.Close()
		}
	}
}

func (c *Client) CodecData() ([]av.CodecData, error) {
	var sps, pps []byte
	var aacCodec *codecs.MPEG4Audio
	for _, track := range c.reader.Tracks() {
		switch codec := track.Codec.(type) {
		case *codecs.H264:
			c.reader.OnDataH264(track, func(pts, dts int64, au [][]byte) error {
				for _, nalu := range au {
					var isKeyFrame bool
					switch h264.NALUType(nalu[0] & 0x1F) {
					case h264.NALUTypeSPS:
						if sps == nil {
							sps = nalu
						}
						continue
					case h264.NALUTypePPS:
						if pps == nil {
							pps = nalu
						}
						continue
					case h264.NALUTypeIDR:
						isKeyFrame = true
						fallthrough
					case h264.NALUTypeNonIDR, h264.NALUTypeDataPartitionA, h264.NALUTypeDataPartitionB, h264.NALUTypeDataPartitionC:
						b := make([]byte, 4+len(nalu))
						binary.BigEndian.PutUint32(b, uint32(len(nalu)))
						copy(b[4:], nalu)
						pts := time.Duration(pts) * time.Second / time.Duration(90000)
						dts := time.Duration(dts) * time.Second / time.Duration(90000)

						c.packetQueue <- &av.Packet{
							Idx:             0,
							IsKeyFrame:      isKeyFrame,
							CompositionTime: pts - dts,
							Time:            dts,
							Data:            b,
						}
					}
				}
				return nil
			})
		case *codecs.MPEG4Audio:
			aacCodec = codec
			c.reader.OnDataMPEG4Audio(track, func(pts int64, aus [][]byte) error {
				for i, au := range aus {
					delta := time.Duration(i) * mpeg4audio.SamplesPerAccessUnit * time.Second / time.Duration(codec.Config.SampleRate)
					c.packetQueue <- &av.Packet{
						Idx:  1,
						Time: (time.Duration(pts) * time.Second / time.Duration(90000)) + delta,
						Data: au,
					}
				}
				return nil
			})
		default:
			return nil, fmt.Errorf("unsupported codec: %T", track.Codec)
		}
	}

	for sps == nil || pps == nil {
		if err := c.reader.Read(); err != nil {
			return nil, err
		}
	}

	var codecs []av.CodecData
	h264Codec, err := h264parser.NewCodecDataFromSPSAndPPS(sps, pps)
	if err != nil {
		return nil, err
	}
	codecs = append(codecs, h264Codec)

	if aacCodec != nil {
		config, err := aacCodec.Config.Marshal()
		if err != nil {
			return nil, err
		}

		aacCodec, err := aacparser.NewCodecDataFromMPEG4AudioConfigBytes(config)
		if err != nil {
			return nil, err
		}
		codecs = append(codecs, aacCodec)
	}

	go func() {
		for {
			if err := c.reader.Read(); err != nil {
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
