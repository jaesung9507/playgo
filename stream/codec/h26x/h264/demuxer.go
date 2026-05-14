package h264

import (
	"fmt"
	"io"
	"time"

	"github.com/jaesung9507/playgo/stream/codec/h26x"

	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/codec/h264parser"
)

const timebase = 90000

type Demuxer struct {
	r        *h26x.NALUReader
	pts      int64
	dts      *DTSExtractor
	duration int64
}

func NewDemuxer(r io.Reader) *Demuxer {
	return &Demuxer{
		r:        h26x.NewNALUReader(r),
		duration: timebase / 25,
		dts:      &DTSExtractor{},
	}
}

func (d *Demuxer) Streams() ([]av.CodecData, error) {
	var sps, pps []byte
	for {
		nalu, err := d.r.Read()
		if err != nil {
			return nil, err
		}
		if len(nalu) == 0 {
			continue
		}

		switch ParseNALUType(nalu[0]) {
		case NALUnitSPS:
			sps = nalu
		case NALUnitPPS:
			pps = nalu
		case NALUnitIDRSlice, NALUnitSlice:
			if sps == nil || pps == nil {
				return nil, fmt.Errorf("missing sps/pps before frame: nalu[0]=%d", nalu[0])
			}
		}

		if sps != nil && pps != nil {
			break
		}
	}

	codec, err := h264parser.NewCodecDataFromSPSAndPPS(sps, pps)
	if err != nil {
		return nil, err
	}

	if fps := int64(codec.FPS()); fps > 0 {
		d.duration = timebase / fps
	}

	d.dts.Initialize()
	d.dts.Extract([][]byte{sps, pps}, 0)

	return []av.CodecData{codec}, nil
}

func (d *Demuxer) readFrame() (au [][]byte, err error) {
	for {
		var nalu []byte
		if nalu, err = d.r.Read(); err != nil {
			return nil, err
		}

		if len(nalu) == 0 {
			continue
		}
		au = append(au, nalu)

		switch ParseNALUType(nalu[0]) {
		case NALUnitIDRSlice, NALUnitSlice:
			return au, nil
		}
	}
}

func (d *Demuxer) ReadPacket() (av.Packet, error) {
	au, err := d.readFrame()
	if err != nil {
		return av.Packet{}, err
	}

	dts, err := d.dts.Extract(au, d.pts)
	if err != nil {
		dts = d.pts
	}

	data, err := AVCC(au).Marshal()
	if err != nil {
		return av.Packet{}, err
	}

	pkt := av.Packet{
		IsKeyFrame: IsKeyFrame(au),
		Data:       data,
		Time:       time.Duration(d.pts) * time.Second / timebase,
	}
	pkt.CompositionTime = pkt.Time - (time.Duration(dts) * time.Second / timebase)
	d.pts += d.duration

	return pkt, nil
}
