package mp4

import (
	"fmt"
	"io"
	"log"
	"time"

	"github.com/jaesung9507/playgo/stream"
	"github.com/jaesung9507/playgo/stream/codec/aac"
	"github.com/jaesung9507/playgo/stream/codec/h26x/h264"
	"github.com/jaesung9507/playgo/stream/codec/h26x/h265"

	"github.com/bluenviron/mediacommon/v2/pkg/formats/mp4/codecs"
	"github.com/bluenviron/mediacommon/v2/pkg/formats/pmp4"
)

type track struct {
	info *pmp4.Track
	pos  int
	dts  int64
}

type Demuxer struct {
	r      io.ReadSeeker
	tracks []*track
}

func NewDemuxer(r io.ReadSeeker) *Demuxer {
	return &Demuxer{
		r: r,
	}
}

func (d *Demuxer) CodecData() ([]stream.Codec, error) {
	presentation := &pmp4.Presentation{}
	if err := presentation.Unmarshal(d.r); err != nil {
		return nil, err
	}

	var result []stream.Codec
	d.tracks = make([]*track, 0, len(presentation.Tracks))
	for i, t := range presentation.Tracks {
		log.Printf("[MP4] on track %d: %T", i, t.Codec)
		d.tracks = append(d.tracks, &track{info: t})
		switch codec := t.Codec.(type) {
		case *codecs.H264:
			result = append(result, &h264.Codec{SPS: codec.SPS, PPS: codec.PPS})
			log.Printf("[MP4] track %d: H264 codec ready", i)
		case *codecs.H265:
			result = append(result, &h265.Codec{VPS: codec.VPS, SPS: codec.SPS, PPS: codec.PPS})
			log.Printf("[MP4] track %d: H265 codec ready", i)
		case *codecs.MPEG4Audio:
			asc, err := codec.Config.Marshal()
			if err != nil {
				return nil, err
			}
			result = append(result, &aac.Codec{ASC: asc, Config: codec.Config})
			log.Printf("[MP4] track %d: AAC codec ready", i)
		default:
			return nil, fmt.Errorf("unsupported codec: %T", codec)
		}
	}

	return result, nil
}

func (d *Demuxer) ReadPacket() (stream.Packet, error) {
	idx := -1
	var ts time.Duration
	for i, t := range d.tracks {
		if t.pos >= len(t.info.Samples) {
			continue
		}

		trackTime := time.Duration(t.dts) * time.Second / time.Duration(t.info.TimeScale)
		if idx < 0 || trackTime < ts {
			idx = i
			ts = trackTime
		}
	}

	if idx < 0 {
		return stream.Packet{}, io.EOF
	}

	t := d.tracks[idx]
	sample := t.info.Samples[t.pos]
	data, err := sample.GetPayload()
	if err != nil {
		return stream.Packet{}, err
	}

	t.pos++
	t.dts += int64(sample.Duration)

	return stream.Packet{
		Idx:             int8(idx),
		IsKeyFrame:      !sample.IsNonSyncSample,
		Time:            ts,
		CompositionTime: time.Duration(sample.PTSOffset) * time.Second / time.Duration(t.info.TimeScale),
		Data:            data,
	}, nil
}
