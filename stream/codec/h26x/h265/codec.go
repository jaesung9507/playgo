package h265

import (
	"fmt"

	"github.com/bluenviron/mediacommon/v2/pkg/codecs/h265"
)

type DTSExtractor = h265.DTSExtractor

type Codec struct {
	VPS []byte
	SPS []byte
	PPS []byte
}

func (c *Codec) CodecString() string {
	base := "hvc1"
	if c != nil && c.SPS != nil {
		var sps h265.SPS
		if err := sps.Unmarshal(c.SPS); err == nil {
			var compat uint32
			for j := range 32 {
				if sps.ProfileTierLevel.GeneralProfileCompatibilityFlag[j] {
					compat |= (1 << uint(j))
				}
			}

			tier := "L"
			if sps.ProfileTierLevel.GeneralTierFlag != 0 {
				tier = "H"
			}

			return fmt.Sprintf("%s.%d.%X.%s%d.B0", base, sps.ProfileTierLevel.GeneralProfileIdc, compat, tier, sps.ProfileTierLevel.GeneralLevelIdc)
		}
	}

	return base
}
