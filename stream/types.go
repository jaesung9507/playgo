package stream

import (
	"time"
)

type Codec interface {
	CodecString() string
}

type Packet struct {
	Idx             int8
	IsKeyFrame      bool
	Time            time.Duration
	CompositionTime time.Duration
	Data            []byte
}

type Demuxer interface {
	CodecData() ([]Codec, error)
	ReadPacket() (Packet, error)
}

type Client interface {
	Dial() error
	Close()
	CodecData() ([]Codec, error)
	PacketQueue() <-chan *Packet
	CloseCh() <-chan any
	Secure() (bool, bool, map[string]string)
}

func IsCodecReady(codecs []Codec) bool {
	for _, codec := range codecs {
		if codec == nil {
			return false
		}
	}

	return true
}
