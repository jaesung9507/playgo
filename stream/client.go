package stream

import (
	"context"
)

type Client interface {
	Dial() error
	Close()
	CodecData() ([]Codec, error)
	PacketQueue() <-chan *Packet
	CloseCh() <-chan any
	Secure() (bool, bool, map[string]string)
}

func Dial(ctx context.Context, c Client) error {
	ch := make(chan error, 1)
	go func() {
		ch <- c.Dial()
	}()

	select {
	case <-ctx.Done():
		go func() {
			<-ch
		}()
		return context.Canceled
	case err := <-ch:
		if err != nil {
			return err
		}
	}

	return nil
}

func CodecData(ctx context.Context, c Client) (codecs []Codec, err error) {
	defer func() {
		if err != nil {
			c.Close()
		}
	}()

	ch := make(chan error, 1)
	go func() {
		var err error
		codecs, err = c.CodecData()
		ch <- err
	}()

	select {
	case <-ctx.Done():
		return nil, context.Canceled
	case err := <-ch:
		if err != nil {
			return nil, err
		}
	}

	return codecs, nil
}
