package h264

import (
	"fmt"

	"github.com/bluenviron/mediacommon/v2/pkg/codecs/h264"
	"github.com/jaesung9507/playgo/stream/codec/h26x"
)

type DTSExtractor = h264.DTSExtractor

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

func (c *Codec) parseAU(au [][]byte, payloadOnly bool) (bool, []byte) {
	payload := au
	if payloadOnly {
		payload = make([][]byte, 0, len(au))
	}

	var isKeyFrame bool
	for _, nalu := range au {
		naluType := ParseNALUType(nalu[0])
		switch naluType {
		case NALUnitSPS:
			c.SPS = nalu
			if payloadOnly {
				continue
			}
		case NALUnitPPS:
			c.PPS = nalu
			if payloadOnly {
				continue
			}
		}

		isKeyFrame = isKeyFrame || naluType.IsKeyFrame()
		if payloadOnly {
			payload = append(payload, nalu)
		}
	}

	data, _ := h26x.AVCC(payload).Marshal()
	return isKeyFrame, data
}

func (c *Codec) ParseAU(au [][]byte) (bool, []byte) {
	return c.parseAU(au, false)
}

func (c *Codec) ParseAUPayload(au [][]byte) (bool, []byte) {
	return c.parseAU(au, true)
}
