package hls

import (
	"crypto/tls"
	"encoding/binary"
	"fmt"
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

type HLSClient struct {
	url         *url.URL
	client      *gohlslib.Client
	signal      chan any
	packetQueue chan *av.Packet
	h264Codec   *codecs.H264
	aacCodec    *codecs.MPEG4Audio
}

func New(parsedUrl *url.URL) *HLSClient {
	return &HLSClient{
		url:         parsedUrl,
		signal:      make(chan any, 1),
		packetQueue: make(chan *av.Packet),
	}
}

func (h *HLSClient) Dial() error {
	h.client = &gohlslib.Client{
		URI: h.url.String(),
		HTTPClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		},
	}

	h.client.OnTracks = func(tracks []*gohlslib.Track) error {
		for _, track := range tracks {
			switch c := track.Codec.(type) {
			case *codecs.H264:
				h.h264Codec = c
				h.client.OnDataH26x(track, func(pts, dts int64, au [][]byte) {
					for _, nalu := range au {
						var isKeyFrame bool
						switch h264.NALUType(nalu[0] & 0x1F) {
						case h264.NALUTypeSPS:
							if h.h264Codec.SPS == nil {
								h.h264Codec.SPS = nalu
								if h.h264Codec.PPS != nil {
									h.signal <- true
								}
							}
							continue
						case h264.NALUTypePPS:
							if h.h264Codec.PPS == nil {
								h.h264Codec.PPS = nalu
								if h.h264Codec.SPS != nil {
									h.signal <- true
								}
							}
							continue
						case h264.NALUTypeIDR:
							isKeyFrame = true
							fallthrough
						case h264.NALUTypeNonIDR, h264.NALUTypeDataPartitionA, h264.NALUTypeDataPartitionB, h264.NALUTypeDataPartitionC:
							b := make([]byte, 4+len(nalu))
							binary.BigEndian.PutUint32(b, uint32(len(nalu)))
							copy(b[4:], nalu)
							pts := time.Duration(pts) * time.Second / time.Duration(track.ClockRate)
							dts := time.Duration(dts) * time.Second / time.Duration(track.ClockRate)

							h.packetQueue <- &av.Packet{
								Idx:             0,
								IsKeyFrame:      isKeyFrame,
								CompositionTime: pts - dts,
								Time:            dts,
								Data:            b,
							}
						}
					}
				})
			case *codecs.MPEG4Audio:
				h.aacCodec = c
				h.client.OnDataMPEG4Audio(track, func(pts int64, aus [][]byte) {
					for i, au := range aus {
						delta := time.Duration(i) * mpeg4audio.SamplesPerAccessUnit * time.Second / time.Duration(c.Config.SampleRate)
						h.packetQueue <- &av.Packet{
							Idx:  1,
							Time: (time.Duration(pts) * time.Second / time.Duration(track.ClockRate)) + delta,
							Data: au,
						}
					}
				})
			default:
				h.signal <- fmt.Errorf("unsupported codec: %T", track.Codec)
			}
		}

		if h.h264Codec != nil && h.h264Codec.SPS != nil && h.h264Codec.PPS != nil {
			h.signal <- true
		}

		return nil
	}

	return h.client.Start()
}

func (h *HLSClient) Close() {
	if h.client != nil {
		h.client.Close()
	}
}

func (h *HLSClient) CodecData() ([]av.CodecData, error) {
	<-h.signal
	var codecs []av.CodecData
	if h.h264Codec != nil && h.h264Codec.SPS != nil && h.h264Codec.PPS != nil {
		h264Codec, err := h264parser.NewCodecDataFromSPSAndPPS(h.h264Codec.SPS, h.h264Codec.PPS)
		if err != nil {
			return nil, err
		}
		codecs = append(codecs, h264Codec)
	}

	if h.aacCodec != nil {
		config, err := h.aacCodec.Config.Marshal()
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
		h.signal <- h.client.Wait2()
	}()

	return codecs, nil
}

func (h *HLSClient) PacketQueue() <-chan *av.Packet {
	return h.packetQueue
}

func (h *HLSClient) CloseCh() <-chan any {
	return h.signal
}
