package cime

import (
	"crypto/tls"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/deepch/vdk/av"
	"github.com/jaesung9507/playgo/stream/protocol/hls"
	httpStream "github.com/jaesung9507/playgo/stream/protocol/http"
)

type Client struct {
	url        *url.URL
	hlsClient  *hls.Client
	httpClient *httpStream.MP4Client
}

func New(parsedURL *url.URL) *Client {
	parsedURL.Path = strings.TrimSuffix(parsedURL.Path, "/")
	return &Client{
		url: parsedURL,
	}
}

func (c *Client) Dial() error {
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	log.Printf("[CEMI] dial: %s", c.url.String())
	var hlsURL, mp4URL *url.URL
	if channelPath, ok := strings.CutPrefix(c.url.Path, "/@"); ok {
		if channelSlug, tailPath, ok := strings.Cut(channelPath, "/"); ok {
			if tailPath == "live" {
				rawURL, err := GetLiveHLSURL(httpClient, channelSlug)
				if err != nil {
					return err
				}

				hlsURL, err = url.Parse(rawURL)
				if err != nil {
					return err
				}
			} else if vodID, ok := strings.CutPrefix(tailPath, "vods/"); ok {
				rawURL, err := GetVODHLSURL(httpClient, channelSlug, vodID)
				if err != nil {
					return err
				}

				hlsURL, err = url.Parse(rawURL)
				if err != nil {
					return err
				}
			}
		}
	} else if clipID, ok := strings.CutPrefix(c.url.Path, "/clips/"); ok {
		rawURL, err := GetClipMP4URL(httpClient, clipID)
		if err != nil {
			return err
		}

		mp4URL, err = url.Parse(rawURL)
		if err != nil {
			return err
		}
	}

	if hlsURL != nil {
		c.hlsClient = hls.New(hlsURL)
		return c.hlsClient.Dial()
	} else if mp4URL != nil {
		c.httpClient = httpStream.NewMP4Client(mp4URL)
		return c.httpClient.Dial()
	}

	return errors.New("not supported url")
}

func (c *Client) Close() {
	log.Print("[CEMI] close")
	if c.hlsClient != nil {
		c.hlsClient.Close()
	}

	if c.httpClient != nil {
		c.httpClient.Close()
	}
}

func (c *Client) CodecData() ([]av.CodecData, error) {
	if c.hlsClient != nil {
		return c.hlsClient.CodecData()
	}

	if c.httpClient != nil {
		return c.httpClient.CodecData()
	}

	return nil, errors.New("not supported")
}

func (c *Client) PacketQueue() <-chan *av.Packet {
	if c.hlsClient != nil {
		return c.hlsClient.PacketQueue()
	}

	if c.httpClient != nil {
		return c.httpClient.PacketQueue()
	}

	return nil
}

func (c *Client) CloseCh() <-chan any {
	if c.hlsClient != nil {
		return c.hlsClient.CloseCh()
	}

	if c.httpClient != nil {
		return c.httpClient.CloseCh()
	}

	return nil
}
