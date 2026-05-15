package h264

import (
	"fmt"

	"github.com/bluenviron/mediacommon/v2/pkg/codecs/h264"
)

type DTSExtractor = h264.DTSExtractor
type AVCC = h264.AVCC

type Codec struct {
	SPS []byte
	PPS []byte

	sps *h264.SPS
}

func (c *Codec) decodeSPS() error {
	if c.sps == nil {
		var sps h264.SPS
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
	base := "avc1"
	if c != nil && len(c.SPS) >= 4 {
		return fmt.Sprintf("%s.%02X%02X%02X", base, c.SPS[1], c.SPS[2], c.SPS[3])
	}

	return base
}
