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

func ParseNALUType(b byte) NALUType {
	return NALUType(b & 0x1F)
}

func IsKeyFrame(au [][]byte) bool {
	for _, nalu := range au {
		if len(nalu) > 0 {
			switch ParseNALUType(nalu[0]) {
			case NALUnitIDRSlice:
				return true
			}
		}
	}

	return false
}
