package h265

type NALUType uint8

const (
	NALUnitTrailN NALUType = iota // 0
	NALUnitTrailR
	NALUnitTSAN
	NALUnitTSAR
	NALUnitSTSAN
	NALUnitSTSAR
	NALUnitRADLN
	NALUnitRADLR
	NALUnitRASLN
	NALUnitRASLR
	NALUnitVCLN10
	NALUnitVCLR11
	NALUnitVCLN12
	NALUnitVCLR13
	NALUnitVCLN14
	NALUnitVCLR15
	NALUnitBLAWLP
	NALUnitBLAWRADL
	NALUnitBLANLP
	NALUnitIDRWRADL
	NALUnitIDRNLP
	NALUnitCRANUT
	NALUnitRSVIRAPVCL22
	NALUnitRSVIRAPVCL23
	NALUnitRSVVCL24
	NALUnitRSVVCL25
	NALUnitRSVVCL26
	NALUnitRSVVCL27
	NALUnitRSVVCL28
	NALUnitRSVVCL29
	NALUnitRSVVCL30
	NALUnitRSVVCL31
	NALUnitVPS
	NALUnitSPS
	NALUnitPPS
)

func ParseNALUType(b byte) NALUType {
	return NALUType((b >> 1) & 0x3F)
}

func IsKeyFrame(au [][]byte) bool {
	for _, nalu := range au {
		if len(nalu) > 0 {
			switch ParseNALUType(nalu[0]) {
			case NALUnitIDRWRADL, NALUnitIDRNLP, NALUnitCRANUT:
				return true
			}
		}
	}

	return false
}
