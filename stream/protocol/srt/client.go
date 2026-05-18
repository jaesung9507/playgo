package srt

import (
	"fmt"
	"log"
	"net/url"
	"reflect"
	"strings"

	"github.com/jaesung9507/playgo/stream"
	"github.com/jaesung9507/playgo/stream/format/ts"

	srt "github.com/datarhei/gosrt"
)

type Client struct {
	url         *url.URL
	conn        srt.Conn
	demuxer     *ts.Demuxer
	signal      chan any
	packetQueue chan *stream.Packet
}

func New(parsedUrl *url.URL) *Client {
	return &Client{
		url:         parsedUrl,
		signal:      make(chan any, 1),
		packetQueue: make(chan *stream.Packet, 128),
	}
}

func (c *Client) getConfig() (*srt.Config, error) {
	cfg := srt.DefaultConfig()
	if _, err := cfg.UnmarshalURL(c.url.String()); err != nil {
		return nil, err
	}

	if len(cfg.StreamId) <= 0 && strings.HasPrefix(c.url.Fragment, "!::") {
		cfg.StreamId = "#" + c.url.Fragment
	}

	return &cfg, nil
}

func (c *Client) Dial() error {
	log.Printf("[SRT] dial: %s", c.url.String())
	cfg, err := c.getConfig()
	if err != nil {
		return err
	}

	c.conn, err = srt.Dial(c.url.Scheme, c.url.Host, *cfg)
	if err != nil {
		return err
	}
	c.demuxer = ts.NewDemuxer(c.conn)

	return nil
}

func (c *Client) Close() {
	log.Print("[SRT] close")
	if c.conn != nil {
		c.conn.Close()
	}
}

func (c *Client) CodecData() ([]stream.Codec, error) {
	codecs, err := c.demuxer.CodecData()
	if err == nil {
		go func() {
			for {
				packet, err := c.demuxer.ReadPacket()
				if err != nil {
					c.signal <- err
					return
				}
				c.packetQueue <- &packet
			}
		}()
	}

	return codecs, err
}

func (c *Client) PacketQueue() <-chan *stream.Packet {
	return c.packetQueue
}

func (c *Client) CloseCh() <-chan any {
	return c.signal
}

func (c *Client) Secure() (bool, bool, map[string]string) {
	crypto := reflect.ValueOf(c.conn).Elem().FieldByName("crypto")
	if crypto.IsValid() && !crypto.IsNil() {
		keyLength := crypto.Elem().Elem().FieldByName("keyLength")
		if keyLength.IsValid() && keyLength.CanInt() {
			return true, true, map[string]string{
				"Cipher": fmt.Sprintf("AES-%d", keyLength.Int()*8),
			}
		}
	}

	return false, false, nil
}
