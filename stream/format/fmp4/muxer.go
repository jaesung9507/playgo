package fmp4

import (
	"fmt"
	"strings"
	"time"

	"github.com/jaesung9507/playgo/stream"
	"github.com/jaesung9507/playgo/stream/codec/aac"
	"github.com/jaesung9507/playgo/stream/codec/h26x/h264"
	"github.com/jaesung9507/playgo/stream/codec/h26x/h265"

	"github.com/bluenviron/mediacommon/v2/pkg/formats/fmp4"
	"github.com/bluenviron/mediacommon/v2/pkg/formats/fmp4/seekablebuffer"
	"github.com/bluenviron/mediacommon/v2/pkg/formats/mp4/codecs"
)

type Muxer struct {
	tracks      []*fmp4.InitTrack
	samples     map[int8][]*fmp4.Sample
	baseTimes   map[int8]uint64
	prevPackets map[int8]*stream.Packet
	sequenceNum uint32
	maxFrames   int
}

func NewMuxer() *Muxer {
	return &Muxer{
		baseTimes:   make(map[int8]uint64),
		samples:     make(map[int8][]*fmp4.Sample),
		prevPackets: make(map[int8]*stream.Packet),
		maxFrames:   5,
	}
}

func (m *Muxer) WriteHeader(codecData []stream.Codec) (string, []byte, error) {
	var (
		tracks       []*fmp4.InitTrack
		codecStrings []string
	)

	for i, codec := range codecData {
		switch codec := codec.(type) {
		case *h264.Codec:
			tracks = append(tracks, &fmp4.InitTrack{
				ID:        i + 1,
				TimeScale: 90000,
				Codec: &codecs.H264{
					SPS: codec.SPS,
					PPS: codec.PPS,
				},
			})
		case *h265.Codec:
			tracks = append(tracks, &fmp4.InitTrack{
				ID:        i + 1,
				TimeScale: 90000,
				Codec: &codecs.H265{
					VPS: codec.VPS,
					SPS: codec.SPS,
					PPS: codec.PPS,
				},
			})
		case *aac.Codec:
			tracks = append(tracks, &fmp4.InitTrack{
				ID:        i + 1,
				TimeScale: uint32(codec.Config.SampleRate),
				Codec: &codecs.MPEG4Audio{
					Config: codec.Config,
				},
			})
		default:
			return "", nil, fmt.Errorf("unsupported codec: %T", codec)
		}
		codecStrings = append(codecStrings, codec.CodecString())
	}

	m.tracks = tracks
	init := &fmp4.Init{
		Tracks: tracks,
	}

	buf := &seekablebuffer.Buffer{}
	if err := init.Marshal(buf); err != nil {
		return "", nil, err
	}

	return strings.Join(codecStrings, ","), buf.Bytes(), nil
}

func (m *Muxer) WritePacket(packet stream.Packet) ([]byte, error) {
	if int(packet.Idx) >= len(m.tracks) {
		return nil, fmt.Errorf("invalid track index: %d", packet.Idx)
	}
	defer func() { m.prevPackets[packet.Idx] = &packet }()

	track := m.tracks[packet.Idx]

	if prev := m.prevPackets[packet.Idx]; prev != nil {
		sample := &fmp4.Sample{
			Duration:        uint32((packet.Time - prev.Time) * time.Duration(track.TimeScale) / time.Second),
			PTSOffset:       int32(prev.CompositionTime * time.Duration(track.TimeScale) / time.Second),
			IsNonSyncSample: !prev.IsKeyFrame,
			Payload:         prev.Data,
		}
		m.samples[packet.Idx] = append(m.samples[packet.Idx], sample)
	}

	if len(m.samples[packet.Idx]) < m.maxFrames {
		return nil, nil
	}

	part := &fmp4.Part{
		SequenceNumber: m.sequenceNum,
		Tracks: []*fmp4.PartTrack{
			{
				ID:       track.ID,
				BaseTime: m.baseTimes[packet.Idx],
				Samples:  m.samples[packet.Idx],
			},
		},
	}

	for _, s := range m.samples[packet.Idx] {
		m.baseTimes[packet.Idx] += uint64(s.Duration)
	}
	m.sequenceNum++
	m.samples[packet.Idx] = nil

	buf := &seekablebuffer.Buffer{}
	if err := part.Marshal(buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
