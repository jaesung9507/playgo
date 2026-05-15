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

	sps *h265.SPS
}

func (c *Codec) decodeSPS() error {
	if c.sps == nil {
		var sps h265.SPS
		err := sps.Unmarshal(c.SPS)
		if err == nil {
			c.sps = &sps
		}

		return err
	}

	return nil
}

func (c *Codec) FPS() float64 {
	if err := c.decodeSPS(); err == nil {
		return c.sps.FPS()
	}

	return 0
}

func (c *Codec) CodecString() string {
	base := "hvc1"
	if err := c.decodeSPS(); err == nil {
		var compat uint32
		for j := range 32 {
			if c.sps.ProfileTierLevel.GeneralProfileCompatibilityFlag[j] {
				compat |= (1 << uint(j))
			}
		}

		tier := "L"
		if c.sps.ProfileTierLevel.GeneralTierFlag != 0 {
			tier = "H"
		}

		return fmt.Sprintf("%s.%d.%X.%s%d.B0", base, c.sps.ProfileTierLevel.GeneralProfileIdc, compat, tier, c.sps.ProfileTierLevel.GeneralLevelIdc)
	}

	return base
}
