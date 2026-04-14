package sbs

import (
	"crypto/tls"
	"errors"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/deepch/vdk/av"
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
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	log.Printf("[SBS] dial: %s", c.url.String())

	var hlsURL *url.URL
	switch c.url.Host {
	case "sbs.co.kr", "www.sbs.co.kr":
		if channelID, ok := strings.CutPrefix(c.url.Path, "/live/"); ok {
			resp, err := GetOnAir(client, channelID)
			if err != nil {
				return err
			}
			log.Printf("[SBS] video title: %s", resp.Info.Title)

			hlsURL, err = url.Parse(resp.HLSURL())
			if err != nil {
				return err
			}
		}
	case "allvod.sbs.co.kr":
		if m := regexp.MustCompile(`^/watch/[^/]+/[^/]+/([^/]+)$`).FindStringSubmatch(c.url.Path); m != nil {
			resp, err := GetVOD(client, m[1])
			if err != nil {
				return err
			}
			log.Printf("[SBS] video title: %s", resp.Info.Title)

			hlsURL, err = url.Parse(resp.HLSURL())
			if err != nil {
				return err
			}
		}
	case "programs.sbs.co.kr":
		if m := regexp.MustCompile(`^/[^/]+/[^/]+/[^/]+/[^/]+/([^/]+)$`).FindStringSubmatch(c.url.Path); m != nil {
			resp, err := GetVOD(client, m[1])
			if err != nil {
				return err
			}
			log.Printf("[SBS] video title: %s", resp.Info.Title)

			hlsURL, err = url.Parse(resp.HLSURL())
			if err != nil {
				return err
			}
		}
	}

	if hlsURL != nil {
		c.hlsClient = hls.New(hlsURL)
		return c.hlsClient.Dial()
	}

	return errors.New("not supported url")
}

func (c *Client) Close() {
	log.Print("[SBS] close")
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
