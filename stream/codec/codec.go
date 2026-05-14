package codec

import (
	"fmt"

	"github.com/jaesung9507/playgo/stream/codec/aac"
	"github.com/jaesung9507/playgo/stream/codec/h26x/h264"
	"github.com/jaesung9507/playgo/stream/codec/h26x/h265"

	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/codec/aacparser"
	"github.com/deepch/vdk/codec/h264parser"
	"github.com/deepch/vdk/codec/h265parser"
)

type Codec interface {
	CodecString() string
}

func VDKCodec2Codecs(vdkCodecs []av.CodecData) ([]Codec, error) {
	var codecs []Codec
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
