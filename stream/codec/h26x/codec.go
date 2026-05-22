package h26x

import "github.com/bluenviron/mediacommon/v2/pkg/codecs/h264"

type AVCC = h264.AVCC

type Codec interface {
	CodecString() string
	Ready() bool
	FPS() float64
	ParseAU(au [][]byte) (bool, []byte)
}

type DTSExtractor interface {
	Extract(au [][]byte, pts int64) (int64, error)
	Initialize()
}
