package srt

import (
	"encoding/binary"
	"fmt"
	"log"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/jaesung9507/playgo/stream/codec"
	"github.com/jaesung9507/playgo/stream/codec/aac"
	"github.com/jaesung9507/playgo/stream/codec/h26x/h264"
	"github.com/jaesung9507/playgo/stream/codec/h26x/h265"

	"github.com/bluenviron/mediacommon/v2/pkg/formats/mpegts"
	"github.com/bluenviron/mediacommon/v2/pkg/formats/mpegts/codecs"
	srt "github.com/datarhei/gosrt"
	"github.com/deepch/vdk/av"
)

type Client struct {
	url         *url.URL
	conn        srt.Conn
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
	log.Printf("[SRT] dial: %s", c.url.String())
	cfg, err := c.getConfig()
	if err != nil {
		return err
	}

	c.conn, err = srt.Dial(c.url.Scheme, c.url.Host, *cfg)
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) Close() {
	log.Print("[SRT] close")
	if c.conn != nil {
		c.conn.Close()
	}
}

func (c *Client) CodecData() ([]codec.Codec, error) {
	reader := &mpegts.Reader{R: c.conn}
	if err := reader.Initialize(); err != nil {
		return nil, err
	}

	tracks := reader.Tracks()
	result := make([]codec.Codec, len(tracks))
	for i, track := range tracks {
		log.Printf("[SRT] on track %d: %T", i, track.Codec)
		switch codec := track.Codec.(type) {
		case *codecs.H264:
			h264Codec := &h264.Codec{}
			reader.OnDataH264(track, func(pts, dts int64, au [][]byte) error {
				for _, nalu := range au {
					var isKeyFrame bool
					switch h264.ParseNALUType(nalu[0]) {
					case h264.NALUnitSPS:
						h264Codec.SPS = nalu
					case h264.NALUnitPPS:
						h264Codec.PPS = nalu
					case h264.NALUnitIDRSlice:
						isKeyFrame = true
						fallthrough
					case h264.NALUnitSlice, h264.NALUnitDPA, h264.NALUnitDPB, h264.NALUnitDPC:
						b := make([]byte, 4+len(nalu))
						binary.BigEndian.PutUint32(b, uint32(len(nalu)))
						copy(b[4:], nalu)
						pts := time.Duration(pts) * time.Second / time.Duration(90000)
						dts := time.Duration(dts) * time.Second / time.Duration(90000)

						c.packetQueue <- &av.Packet{
							Idx:             int8(i),
							IsKeyFrame:      isKeyFrame,
							CompositionTime: pts - dts,
							Time:            dts,
							Data:            b,
						}
					}
				}

				if result[i] == nil && h264Codec.SPS != nil && h264Codec.PPS != nil {
					result[i] = h264Codec
					log.Printf("[SRT] track %d: H264 codec ready", i)
				}

				return nil
			})
		case *codecs.H265:
			h265Codec := &h265.Codec{}
			reader.OnDataH265(track, func(pts, dts int64, au [][]byte) error {
				for _, nalu := range au {
					var isKeyFrame bool
					naluType := h265.NALUType((nalu[0] >> 1) & 0x3F)
					switch naluType {
					case h265.NALUnitVPS:
						h265Codec.VPS = nalu
					case h265.NALUnitSPS:
						h265Codec.SPS = nalu
					case h265.NALUnitPPS:
						h265Codec.PPS = nalu
					case h265.NALUnitIDRWRADL, h265.NALUnitIDRNLP, h265.NALUnitCRANUT:
						isKeyFrame = true
						fallthrough
					default:
						if naluType <= 31 {
							b := make([]byte, 4+len(nalu))
							binary.BigEndian.PutUint32(b, uint32(len(nalu)))
							copy(b[4:], nalu)
							pts := time.Duration(pts) * time.Second / time.Duration(90000)
							dts := time.Duration(dts) * time.Second / time.Duration(90000)

							c.packetQueue <- &av.Packet{
								Idx:             int8(i),
								IsKeyFrame:      isKeyFrame,
								CompositionTime: pts - dts,
								Time:            dts,
								Data:            b,
							}
						}
					}
				}

				if result[i] == nil && h265Codec.VPS != nil && h265Codec.SPS != nil && h265Codec.PPS != nil {
					result[i] = h265Codec
					log.Printf("[SRT] track %d: H265 codec ready", i)
				}

				return nil
			})
		case *codecs.MPEG4Audio:
			asc, err := codec.Config.Marshal()
			if err != nil {
				return nil, err
			}
			result[i] = &aac.Codec{ASC: asc, Config: codec.Config}
			log.Printf("[SRT] track %d: AAC codec ready", i)

			reader.OnDataMPEG4Audio(track, func(pts int64, aus [][]byte) error {
				for j, au := range aus {
					delta := time.Duration(j) * aac.SamplesPerAccessUnit * time.Second / time.Duration(codec.Config.SampleRate)
					c.packetQueue <- &av.Packet{
						Idx:  int8(i),
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

	isReady := func() bool {
		for _, r := range result {
			if r == nil {
				return false
			}
		}
		return true
	}

	for !isReady() {
		if err := reader.Read(); err != nil {
			return nil, err
		}
	}

	go func() {
		for {
			if err := reader.Read(); err != nil {
				c.signal <- err
				return
			}
		}
	}()

	return result, nil
}

func (c *Client) PacketQueue() <-chan *av.Packet {
	return c.packetQueue
}

func (c *Client) CloseCh() <-chan any {
	return c.signal
}

func (c *Client) Secure() (bool, bool, map[string]string) {
	crypto := reflect.ValueOf(c.conn).Elem().FieldByName("crypto")
	if crypto.IsValid() && !crypto.IsNil() {
		keyLength := crypto.Elem().Elem().FieldByName("keyLength")
		if keyLength.IsValid() && keyLength.CanInt() {
			return true, true, map[string]string{
				"Cipher": fmt.Sprintf("AES-%d", keyLength.Int()*8),
			}
		}
	}

	return false, false, nil
}
