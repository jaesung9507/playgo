package pandatv

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/jaesung9507/playgo/secure"
	"github.com/jaesung9507/playgo/stream/protocol/hls"

	"github.com/deepch/vdk/av"
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
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tls.Config(),
		},
	}

	log.Printf("[PandaTV] dial: %s", c.url.String())
	var hlsURL *url.URL
	if userID, ok := strings.CutPrefix(c.url.Path, "/play/"); ok {
		rawURL, err := GetLiveHLSURL(client, userID)
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
		return c.hlsClient.DialWithHeader(map[string]string{
			"Origin": "https://www.pandalive.co.kr",
		})
	}

	return errors.New("not supported url")
}

func (c *Client) Close() {
	log.Print("[PandaTV] close")
	if c.hlsClient != nil {
		c.hlsClient.Close()
	}
}

func (c *Client) CodecData() ([]av.CodecData, error) {
	if c.hlsClient != nil {
		return c.hlsClient.CodecData()
	}

	return nil, errors.New("not supported")
}

func (c *Client) PacketQueue() <-chan *av.Packet {
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
