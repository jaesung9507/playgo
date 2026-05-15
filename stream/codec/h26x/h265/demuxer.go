package h265

import (
	"bytes"
	"encoding/binary"
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
		case NALUnitVPS:
			c.VPS = nalu
		case NALUnitSPS:
			c.SPS = nalu
		case NALUnitPPS:
			c.PPS = nalu
		default:
			if c.VPS == nil || c.SPS == nil || c.PPS == nil {
				return nil, fmt.Errorf("missing vps/sps/pps before frame: nalu[0]=%d", nalu[0])
			}
		}

		if c.VPS != nil && c.SPS != nil && c.PPS != nil {
			break
		}
	}

	if fps := c.FPS(); fps > 0 {
		d.duration = int64(float64(timebase) / fps)
	}

	d.dts.Initialize()
	d.dts.Extract([][]byte{c.VPS, c.SPS, c.PPS}, 0)

	return []stream.Codec{&c}, nil
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

		if ParseNALUType(nalu[0]) <= 31 {
			return au, nil
		}
	}
}

func (d *Demuxer) ReadPacket() (stream.Packet, error) {
	au, err := d.readFrame()
	if err != nil {
		return stream.Packet{}, err
	}

	dts, err := d.dts.Extract(au, d.pts)
	if err != nil {
		dts = d.pts
	}

	buf := bytes.NewBuffer(nil)
	for _, nalu := range au {
		binary.Write(buf, binary.BigEndian, uint32(len(nalu)))
		buf.Write(nalu)
	}

	pkt := stream.Packet{
		IsKeyFrame: IsKeyFrame(au),
		Data:       buf.Bytes(),
		Time:       time.Duration(d.pts) * time.Second / timebase,
	}
	pkt.CompositionTime = pkt.Time - (time.Duration(dts) * time.Second / timebase)
	d.pts += d.duration

	return pkt, nil
}
