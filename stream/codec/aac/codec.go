package aac

import (
	"fmt"

	"github.com/bluenviron/mediacommon/v2/pkg/codecs/mpeg4audio"
)

const SamplesPerAccessUnit = mpeg4audio.SamplesPerAccessUnit

type Codec struct {
	ASC []byte

	Config mpeg4audio.AudioSpecificConfig
}

func (c *Codec) Decode() error {
	return c.Config.Unmarshal(c.ASC)
}

func (c *Codec) CodecString() string {
	return fmt.Sprintf("mp4a.40.%d", c.Config.Type)
}
