package vdk

import (
	"fmt"

	"github.com/jaesung9507/playgo/stream"
	"github.com/jaesung9507/playgo/stream/codec/aac"
	"github.com/jaesung9507/playgo/stream/codec/h26x/h264"
	"github.com/jaesung9507/playgo/stream/codec/h26x/h265"

	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/codec/aacparser"
	"github.com/deepch/vdk/codec/h264parser"
	"github.com/deepch/vdk/codec/h265parser"
)

type Demuxer struct {
	d av.Demuxer
}

func (d *Demuxer) CodecData() ([]stream.Codec, error) {
	codecs, err := d.d.Streams()
	if err != nil {
		return nil, err
	}

	return ToCodecs(codecs)
}

func (d *Demuxer) ReadPacket() (stream.Packet, error) {
	p, err := d.d.ReadPacket()
	if err != nil {
		return stream.Packet{}, err
	}

	return stream.Packet{
		Idx:             p.Idx,
		IsKeyFrame:      p.IsKeyFrame,
		Time:            p.Time,
		CompositionTime: p.CompositionTime,
		Data:            p.Data,
	}, nil
}

func ToDemuxer(d av.Demuxer) stream.Demuxer {
	return &Demuxer{d: d}
}

func ToCodecs(vdkCodecs []av.CodecData) ([]stream.Codec, error) {
	var codecs []stream.Codec
	for _, vdkCodec := range vdkCodecs {
		switch vdkCodec := vdkCodec.(type) {
		case h264parser.CodecData:
			codecs = append(codecs, &h264.Codec{SPS: vdkCodec.SPS(), PPS: vdkCodec.PPS()})
		case h265parser.CodecData:
			codecs = append(codecs, &h265.Codec{VPS: vdkCodec.VPS(), SPS: vdkCodec.SPS(), PPS: vdkCodec.PPS()})
		case aacparser.CodecData:
			aacCodec := &aac.Codec{ASC: vdkCodec.ConfigBytes}
			if err := aacCodec.Decode(); err != nil {
				return nil, err
			}
			codecs = append(codecs, aacCodec)
		default:
			return nil, fmt.Errorf("not supported codec: %T", vdkCodec)
		}
	}

	return codecs, nil
}
