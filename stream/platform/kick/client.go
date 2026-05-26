package kick

import (
	"errors"
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
	return &Client{
		url: parsedURL,
	}
}

func (c *Client) Dial() error {
	var tls secure.TLS
	client := tls.HTTPClient()
	log.Printf("[KICK] dial: %s", c.url.String())
	var hlsURL *url.URL
	if slug, ok := strings.CutPrefix(c.url.Path, "/"); ok {
		rawURL, err := GetLiveHLSURL(client, slug)
		if err != nil {
			return err
		}

		hlsURL, err = url.Parse(rawURL)
		if err != nil {
			return err
		}
	}

	if hlsURL != nil {
		c.hlsClient = hls.New(hlsURL)
		return c.hlsClient.Dial()
	}

	return errors.New("not supported url")
}

func (c *Client) Close() {
	log.Print("[KICK] close")
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
