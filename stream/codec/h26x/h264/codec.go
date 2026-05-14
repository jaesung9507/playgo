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
}

func (c *Codec) CodecString() string {
	base := "avc1"
	if c != nil && len(c.SPS) >= 4 {
		return fmt.Sprintf("%s.%02X%02X%02X", base, c.SPS[1], c.SPS[2], c.SPS[3])
	}

	return base
}
