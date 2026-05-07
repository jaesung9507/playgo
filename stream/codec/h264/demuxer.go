package h264

import (
	"errors"
	"io"
	"time"

	"github.com/bluenviron/mediacommon/v2/pkg/codecs/h264"
	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/codec/h264parser"
)

type Demuxer struct {
	r        io.Reader
	nalus    [][]byte
	pos      int
	pts      int64
	dts      *h264.DTSExtractor
	duration int64
}

func NewDemuxer(r io.Reader) *Demuxer {
	return &Demuxer{
		r:        r,
		duration: 3600,
		dts:      &h264.DTSExtractor{},
	}
}

func (d *Demuxer) Streams() ([]av.CodecData, error) {
	if d.nalus == nil {
		data, err := io.ReadAll(d.r)
		if err != nil {
			return nil, err
		}
		d.nalus, _ = h264parser.SplitNALUs(data)
	}

	var sps, pps []byte
	for _, nalu := range d.nalus {
		if len(nalu) == 0 {
			continue
		}
		typ := h264.NALUType(nalu[0] & 0x1F)
		switch typ {
		case h264.NALUTypeSPS:
			sps = nalu
		case h264.NALUTypePPS:
			pps = nalu
		}
		if sps != nil && pps != nil {
			break
		}
	}

	if sps == nil || pps == nil {
		return nil, errors.New("not found sps/pps")
	}

	codec, err := h264parser.NewCodecDataFromSPSAndPPS(sps, pps)
	if err != nil {
		return nil, err
	}

	if fps := int64(codec.FPS()); fps > 0 {
		d.duration = 90000 / fps
	}

	d.dts.Initialize()

	return []av.CodecData{codec}, nil
}

func (d *Demuxer) read() (au [][]byte, err error) {
	if d.pos >= len(d.nalus) {
		return nil, io.EOF
	}

	for d.pos < len(d.nalus) {
		nalu := d.nalus[d.pos]
		if len(nalu) == 0 {
			d.pos++
			continue
		}

		au = append(au, nalu)
		d.pos++

		switch h264.NALUType(nalu[0] & 0x1F) {
		case h264.NALUTypeIDR, h264.NALUTypeNonIDR:
			return au, nil
		}
	}

	return au, nil
}

func (d *Demuxer) ReadPacket() (av.Packet, error) {
	au, err := d.read()
	if err != nil {
		return av.Packet{}, err
	}

	dts, err := d.dts.Extract(au, d.pts)
	if err != nil {
		dts = d.pts
	}

	data, err := h264.AVCC(au).Marshal()
	if err != nil {
		return av.Packet{}, err
	}

	pkt := av.Packet{
		IsKeyFrame: h264.IsRandomAccess(au),
		Data:       data,
		Time:       time.Duration(d.pts) * time.Second / 90000,
	}
	pkt.CompositionTime = pkt.Time - (time.Duration(dts) * time.Second / 90000)
	d.pts += d.duration

	return pkt, nil
}
