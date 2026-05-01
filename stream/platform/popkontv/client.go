package popkontv

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/jaesung9507/playgo/secure"
	"github.com/jaesung9507/playgo/stream/protocol/hls"
	httpStream "github.com/jaesung9507/playgo/stream/protocol/http"

	"github.com/deepch/vdk/av"
)

type Client struct {
	url       *url.URL
	hlsClient *hls.Client
	mp4Client *httpStream.Client
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

	log.Printf("[PopkonTV] dial: %s", c.url.String())
	var hlsURL, mp4URL *url.URL
	if strings.HasPrefix(c.url.Path, "/live/view") {
		live, err := GetLiveInfo(client, c.url.String())
		if err != nil {
			return err
		}
		log.Printf("[PopkonTV] video title: %s", live.Title)

		rawURL, err := live.GetHLSURL(client)
		if err != nil {
			return err
		}

		hlsURL, err = url.Parse(rawURL)
		if err != nil {
			return err
		}
	} else if strings.HasPrefix(c.url.Path, "/clip/") {
		clip, err := GetClipInfo(client, c.url.String())
		if err != nil {
			return err
		}
		log.Printf("[PopkonTV] video title: %s", clip.Title)

		mp4URL, err = url.Parse(clip.Address)
		if err != nil {
			return err
		}
	}

	if hlsURL != nil {
		c.hlsClient = hls.New(hlsURL)
		return c.hlsClient.Dial()
	} else if mp4URL != nil {
		c.mp4Client = httpStream.New(mp4URL)
		return c.mp4Client.Dial()
	}

	return errors.New("not supported url")
}

func (c *Client) Close() {
	log.Print("[PopkonTV] close")
	if c.hlsClient != nil {
		c.hlsClient.Close()
	}

	if c.mp4Client != nil {
		c.mp4Client.Close()
	}
}

func (c *Client) CodecData() ([]av.CodecData, error) {
	if c.hlsClient != nil {
		return c.hlsClient.CodecData()
	}

	if c.mp4Client != nil {
		return c.mp4Client.CodecData()
	}

	return nil, errors.New("not supported")
}

func (c *Client) PacketQueue() <-chan *av.Packet {
	if c.hlsClient != nil {
		return c.hlsClient.PacketQueue()
	}

	if c.mp4Client != nil {
		return c.mp4Client.PacketQueue()
	}

	return nil
}

func (c *Client) CloseCh() <-chan any {
	if c.hlsClient != nil {
		return c.hlsClient.CloseCh()
	}

	if c.mp4Client != nil {
		return c.mp4Client.CloseCh()
	}

	return nil
}

func (c *Client) Secure() (bool, bool, map[string]string) {
	if c.hlsClient != nil {
		return c.hlsClient.Secure()
	}

	if c.mp4Client != nil {
		return c.mp4Client.Secure()
	}

	return false, false, nil
}
