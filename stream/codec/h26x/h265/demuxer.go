package h265

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"github.com/jaesung9507/playgo/stream/codec/h26x"

	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/codec/h265parser"
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
	var vps, sps, pps []byte
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
			vps = nalu
		case NALUnitSPS:
			sps = nalu
		case NALUnitPPS:
			pps = nalu
		default:
			if vps == nil || sps == nil || pps == nil {
				return nil, fmt.Errorf("missing vps/sps/pps before frame: nalu[0]=%d", nalu[0])
			}
		}

		if vps != nil && sps != nil && pps != nil {
			break
		}
	}

	codec, err := h265parser.NewCodecDataFromVPSAndSPSAndPPS(vps, sps, pps)
	if err != nil {
		return nil, err
	}

	if fps := int64(codec.FPS()); fps > 0 {
		d.duration = timebase / fps
	}

	d.dts.Initialize()
	d.dts.Extract([][]byte{vps, sps, pps}, 0)

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

		if ParseNALUType(nalu[0]) <= 31 {
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

	buf := bytes.NewBuffer(nil)
	for _, nalu := range au {
		binary.Write(buf, binary.BigEndian, uint32(len(nalu)))
		buf.Write(nalu)
	}

	pkt := av.Packet{
		IsKeyFrame: IsKeyFrame(au),
		Data:       buf.Bytes(),
		Time:       time.Duration(d.pts) * time.Second / timebase,
	}
	pkt.CompositionTime = pkt.Time - (time.Duration(dts) * time.Second / timebase)
	d.pts += d.duration

	return pkt, nil
}
