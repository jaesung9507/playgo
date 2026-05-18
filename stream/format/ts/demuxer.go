package ts

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"slices"
	"time"

	"github.com/jaesung9507/playgo/stream"
	"github.com/jaesung9507/playgo/stream/codec/aac"
	"github.com/jaesung9507/playgo/stream/codec/h26x/h264"
	"github.com/jaesung9507/playgo/stream/codec/h26x/h265"

	"github.com/bluenviron/mediacommon/v2/pkg/formats/mpegts"
	"github.com/bluenviron/mediacommon/v2/pkg/formats/mpegts/codecs"
)

type Demuxer struct {
	r *mpegts.Reader
	q []stream.Packet
}

func NewDemuxer(r io.Reader) *Demuxer {
	return &Demuxer{
		r: &mpegts.Reader{R: r},
	}
}

func (d *Demuxer) CodecData() ([]stream.Codec, error) {
	if err := d.r.Initialize(); err != nil {
		return nil, err
	}

	tracks := d.r.Tracks()
	result := make([]stream.Codec, len(tracks))
	for i, track := range tracks {
		log.Printf("[MPEG-TS] on track %d: %T", i, track.Codec)
		switch codec := track.Codec.(type) {
		case *codecs.H264:
			h264Codec := &h264.Codec{}
			d.r.OnDataH264(track, func(pts, dts int64, au [][]byte) error {
				isKeyFrame, data := h264Codec.ParseAU(au)
				if result[i] == nil && h264Codec.SPS != nil && h264Codec.PPS != nil {
					result[i] = h264Codec
					log.Printf("[MPEG-TS] track %d: H264 codec ready", i)
				}

				if len(data) > 0 {
					pts := time.Duration(pts) * time.Second / time.Duration(90000)
					dts := time.Duration(dts) * time.Second / time.Duration(90000)
					d.q = append(d.q, stream.Packet{
						Idx:             int8(i),
						IsKeyFrame:      isKeyFrame,
						CompositionTime: pts - dts,
						Time:            dts,
						Data:            data,
					})
				}

				return nil
			})
		case *codecs.H265:
			h265Codec := &h265.Codec{}
			buf := bytes.NewBuffer(nil)
			d.r.OnDataH265(track, func(pts, dts int64, au [][]byte) error {
				buf.Reset()
				var isKeyFrame bool
				for _, nalu := range au {
					naluType := h265.ParseNALUType(nalu[0])
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
						if naluType <= h265.NALUnitRSVVCL31 {
							binary.Write(buf, binary.BigEndian, uint32(len(nalu)))
							buf.Write(nalu)
						}
					}
				}

				if result[i] == nil && h265Codec.VPS != nil && h265Codec.SPS != nil && h265Codec.PPS != nil {
					result[i] = h265Codec
					log.Printf("[MPEG-TS] track %d: H265 codec ready", i)
				}

				if buf.Len() > 0 {
					pts := time.Duration(pts) * time.Second / time.Duration(90000)
					dts := time.Duration(dts) * time.Second / time.Duration(90000)
					d.q = append(d.q, stream.Packet{
						Idx:             int8(i),
						IsKeyFrame:      isKeyFrame,
						CompositionTime: pts - dts,
						Time:            dts,
						Data:            slices.Clone(buf.Bytes()),
					})
				}

				return nil
			})
		case *codecs.MPEG4Audio:
			asc, err := codec.Config.Marshal()
			if err != nil {
				return nil, err
			}
			result[i] = &aac.Codec{ASC: asc, Config: codec.Config}
			log.Printf("[MPEG-TS] track %d: AAC codec ready", i)

			d.r.OnDataMPEG4Audio(track, func(pts int64, aus [][]byte) error {
				for j, au := range aus {
					delta := time.Duration(j) * aac.SamplesPerAccessUnit * time.Second / time.Duration(codec.Config.SampleRate)
					d.q = append(d.q, stream.Packet{
						Idx:  int8(i),
						Time: (time.Duration(pts) * time.Second / time.Duration(90000)) + delta,
						Data: au,
					})
				}

				return nil
			})
		default:
			return nil, fmt.Errorf("unsupported codec: %T", track.Codec)
		}
	}

	for !stream.IsCodecReady(result) {
		if err := d.r.Read(); err != nil {
			return nil, err
		}
	}

	return result, nil
}

func (d *Demuxer) ReadPacket() (stream.Packet, error) {
	for len(d.q) <= 0 {
		if err := d.r.Read(); err != nil {
			return stream.Packet{}, err
		}
	}

	packet := d.q[0]
	d.q = d.q[1:]

	return packet, nil
}
