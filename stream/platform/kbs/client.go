package kbs

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"

	"github.com/jaesung9507/playgo/secure"
	"github.com/jaesung9507/playgo/stream"
	"github.com/jaesung9507/playgo/stream/protocol/hls"
)

type Client struct {
	url       *url.URL
	hlsClient *hls.Client
}

func New(parsedURL *url.URL) *Client {
	parsedURL.Path = strings.TrimSuffix(parsedURL.Path, "/")
	return &Client{
		url: parsedURL,
	}
}

func (c *Client) Dial() error {
	client := (&secure.TLS{}).HTTPClient()
	log.Printf("[KBS] dial: %s", c.url.String())
	var hlsURL *url.URL
	switch c.url.Host {
	case "onair.kbs.co.kr":
		chCode := c.url.Query().Get("ch_code")
		if chCode == "" {
			chCode = "11"
		}

		groupCode := c.url.Query().Get("group_code")
		if groupCode != "" {
			chCode = fmt.Sprintf("%s_%s", groupCode, chCode)
		}

		chs, err := GetChannels(client, chCode)
		if err != nil {
			return err
		}

		for _, ch := range chs {
			if ch.MediaType == "radio" && len(chs) > 1 {
				continue
			}

			hlsURL, err = url.Parse(ch.ServiceURL)
			if err != nil {
				return err
			}
			break
		}
	}

	if hlsURL != nil {
		c.hlsClient = hls.New(hlsURL)
		return c.hlsClient.Dial()
	}

	return errors.New("not supported url")
}

func (c *Client) Close() {
	log.Print("[KBS] close")
	if c.hlsClient != nil {
		c.hlsClient.Close()
	}
}

func (c *Client) CodecData() ([]stream.Codec, error) {
	if c.hlsClient != nil {
		return c.hlsClient.CodecData()
	}

	return nil, errors.New("not supported")
}

func (c *Client) PacketQueue() <-chan *stream.Packet {
	if c.hlsClient != nil {
		return c.hlsClient.PacketQueue()
	}

	return nil
}

func (c *Client) CloseCh() <-chan any {
	if c.hlsClient != nil {
		return c.hlsClient.CloseCh()
	}

	return nil
}

func (c *Client) Secure() (bool, bool, map[string]string) {
	if c.hlsClient != nil {
		return c.hlsClient.Secure()
	}

	return false, false, nil
}
