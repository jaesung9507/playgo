package h265

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/bluenviron/mediacommon/v2/pkg/codecs/h265"
	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/codec/h265parser"
)

const timebase = 90000

type Demuxer struct {
	r        *bufio.Reader
	pts      int64
	dts      *h265.DTSExtractor
	duration int64
}

func NewDemuxer(r io.Reader) *Demuxer {
	return &Demuxer{
		r:        bufio.NewReader(r),
		duration: timebase / 25,
		dts:      &h265.DTSExtractor{},
	}
}

func (d *Demuxer) Streams() ([]av.CodecData, error) {
	var vps, sps, pps []byte
	for {
		nalu, err := d.readNALU()
		if err != nil {
			return nil, err
		}
		if len(nalu) == 0 {
			continue
		}

		switch h265.NALUType((nalu[0] >> 1) & 0x3F) {
		case h265.NALUType_VPS_NUT:
			vps = nalu
		case h265.NALUType_SPS_NUT:
			sps = nalu
		case h265.NALUType_PPS_NUT:
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

		if h265.NALUType((nalu[0]>>1)&0x3F) <= 31 {
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
		IsKeyFrame: h265.IsRandomAccess(au),
		Data:       buf.Bytes(),
		Time:       time.Duration(d.pts) * time.Second / timebase,
	}
	pkt.CompositionTime = pkt.Time - (time.Duration(dts) * time.Second / timebase)
	d.pts += d.duration

	return pkt, nil
}
