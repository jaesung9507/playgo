package h264

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/bluenviron/mediacommon/v2/pkg/codecs/h264"
	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/codec/h264parser"
)

const timebase = 90000

type Demuxer struct {
	r        *bufio.Reader
	pts      int64
	dts      *h264.DTSExtractor
	duration int64
}

func NewDemuxer(r io.Reader) *Demuxer {
	return &Demuxer{
		r:        bufio.NewReader(r),
		duration: timebase / 25,
		dts:      &h264.DTSExtractor{},
	}
}

func (d *Demuxer) Streams() ([]av.CodecData, error) {
	var sps, pps []byte
	for {
		nalu, err := d.readNALU()
		if err != nil {
			return nil, err
		}
		if len(nalu) == 0 {
			continue
		}

		switch h264.NALUType(nalu[0] & 0x1F) {
		case h264.NALUTypeSPS:
			sps = nalu
		case h264.NALUTypePPS:
			pps = nalu
		case h264.NALUTypeIDR, h264.NALUTypeNonIDR:
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

func (d *Demuxer) readNALU() ([]byte, error) {
	for {
		p, err := d.r.Peek(4)
		if err != nil {
			return nil, err
		}

		if len(p) >= 3 && p[0] == 0 && p[1] == 0 {
			if p[2] == 1 {
				d.r.Discard(3)
				break
			}
			if len(p) >= 4 && p[2] == 0 && p[3] == 1 {
				d.r.Discard(4)
				break
			}
		}
		d.r.Discard(1)
	}

	var nalu []byte
	for {
		p, err := d.r.Peek(4)
		if len(p) >= 3 && p[0] == 0 && p[1] == 0 && (p[2] == 1 || (len(p) >= 4 && p[2] == 0 && p[3] == 1)) {
			break
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				buf := make([]byte, d.r.Buffered())
				n, _ := d.r.Read(buf)
				nalu = append(nalu, buf[:n]...)
			}
			if len(nalu) == 0 {
				return nil, err
			}
			break
		}

		b, _ := d.r.ReadByte()
		nalu = append(nalu, b)
	}

	return nalu, nil
}

func (d *Demuxer) readFrame() (au [][]byte, err error) {
	for {
		var nalu []byte
		if nalu, err = d.readNALU(); err != nil {
			return nil, err
		}

		if len(nalu) == 0 {
			continue
		}
		au = append(au, nalu)

		switch h264.NALUType(nalu[0] & 0x1F) {
		case h264.NALUTypeIDR, h264.NALUTypeNonIDR:
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

	data, err := h264.AVCC(au).Marshal()
	if err != nil {
		return av.Packet{}, err
	}

	pkt := av.Packet{
		IsKeyFrame: h264.IsRandomAccess(au),
		Data:       data,
		Time:       time.Duration(d.pts) * time.Second / timebase,
	}
	pkt.CompositionTime = pkt.Time - (time.Duration(dts) * time.Second / timebase)
	d.pts += d.duration

	return pkt, nil
}
