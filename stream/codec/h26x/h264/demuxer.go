package h264

import (
	"fmt"
	"io"
	"time"

	"github.com/jaesung9507/playgo/stream"
	"github.com/jaesung9507/playgo/stream/codec/h26x"
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

func (d *Demuxer) CodecData() ([]stream.Codec, error) {
	var c Codec
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
			c.SPS = nalu
		case NALUnitPPS:
			c.PPS = nalu
		case NALUnitIDRSlice, NALUnitSlice:
			if c.SPS == nil || c.PPS == nil {
				return nil, fmt.Errorf("missing sps/pps before frame: nalu[0]=%d", nalu[0])
			}
		}

		if c.SPS != nil && c.PPS != nil {
			break
		}
	}

	if fps := c.FPS(); fps > 0 {
		d.duration = int64(float64(timebase) / fps)
	}

	d.dts.Initialize()
	d.dts.Extract([][]byte{c.SPS, c.PPS}, 0)

	return []stream.Codec{&c}, nil
}

func (d *Demuxer) readFrame() (isKeyFrame bool, au [][]byte, err error) {
	for {
		var nalu []byte
		if nalu, err = d.r.Read(); err != nil {
			return false, nil, err
		}

		if len(nalu) == 0 {
			continue
		}
		au = append(au, nalu)

		naluType := ParseNALUType(nalu[0])
		switch naluType {
		case NALUnitIDRSlice, NALUnitSlice:
			return naluType.IsKeyFrame(), au, nil
		}
	}
}

func (d *Demuxer) ReadPacket() (stream.Packet, error) {
	isKeyFrame, au, err := d.readFrame()
	if err != nil {
		return stream.Packet{}, err
	}

	dts, err := d.dts.Extract(au, d.pts)
	if err != nil {
		dts = d.pts
	}

	data, err := h26x.AVCC(au).Marshal()
	if err != nil {
		return stream.Packet{}, err
	}

	pkt := stream.Packet{
		IsKeyFrame: isKeyFrame,
		Data:       data,
		Time:       time.Duration(d.pts) * time.Second / timebase,
	}
	pkt.CompositionTime = pkt.Time - (time.Duration(dts) * time.Second / timebase)
	d.pts += d.duration

	return pkt, nil
}
