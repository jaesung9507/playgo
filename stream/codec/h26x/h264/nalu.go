package h264

type NALUType uint8

const (
	NALUnitUnspecified NALUType = iota
	NALUnitSlice
	NALUnitDPA
	NALUnitDPB
	NALUnitDPC
	NALUnitIDRSlice
	NALUnitSEI
	NALUnitSPS
	NALUnitPPS
	NALUnitAUD
	NALUnitEndSequence
	NALUnitEndStream
)

func (t NALUType) IsKeyFrame() bool {
	switch t {
	case NALUnitIDRSlice:
		return true
	}

	return false
}

func ParseNALUType(b byte) NALUType {
	return NALUType(b & 0x1F)
}
