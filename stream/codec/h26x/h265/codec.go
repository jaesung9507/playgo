package h265

import (
	"fmt"

	"github.com/jaesung9507/playgo/stream/codec/h26x"

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

func (c *Codec) Ready() bool {
	return c.VPS != nil && c.SPS != nil && c.PPS != nil
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

func (c *Codec) ParseAU(au [][]byte) (bool, []byte) {
	payload := make([][]byte, 0, len(au))
	var isKeyFrame bool
	for _, nalu := range au {
		naluType := ParseNALUType(nalu[0])
		switch naluType {
		case NALUnitVPS:
			c.VPS = nalu
		case NALUnitSPS:
			c.SPS = nalu
		case NALUnitPPS:
			c.PPS = nalu
		}

		if naluType <= NALUnitRSVVCL31 {
			isKeyFrame = isKeyFrame || naluType.IsKeyFrame()
			payload = append(payload, nalu)
		}
	}

	data, _ := h26x.AVCC(payload).Marshal()
	return isKeyFrame, data
}
