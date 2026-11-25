package kbs

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"strings"

	"github.com/jaesung9507/playgo/secure"
	"github.com/jaesung9507/playgo/stream"
	"github.com/jaesung9507/playgo/stream/format/mp4"
	"github.com/jaesung9507/playgo/stream/protocol/hls"
	"github.com/jaesung9507/playgo/stream/protocol/http"
)

type Client struct {
	url       *url.URL
	hlsClient *hls.Client
	mp4Client *http.MP4Client
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
	var hlsURL, mp4URL *url.URL
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
	case "vod.kbs.co.kr":
		contents, err := GetContents(client, c.url.String())
		if err != nil {
			return err
		}
		log.Printf("[KBS] video title: %s", contents.ContentsTitle())

		if contents.CDNFileName != "" {
			hlsURL, err = url.Parse(contents.CDNFileName)
			if err != nil {
				return err
			}
		} else if contents.Play1URL != "" {
			mp4URL, err = url.Parse(contents.Play1URL)
			if err != nil {
				return err
			}
		} else {
			return errors.New("not found media url")
		}
	}

	if hlsURL != nil {
		c.hlsClient = hls.New(hlsURL)
		return c.hlsClient.Dial()
	}

	if mp4URL != nil {
		c.mp4Client = http.NewMP4Client(mp4URL)
		c.mp4Client.SetNewDemuxer(func(r io.ReadSeeker) stream.Demuxer {
			return mp4.NewDemuxer(r)
		})
		return c.mp4Client.Dial()
	}

	return errors.New("not supported url")
}

func (c *Client) Close() {
	log.Print("[KBS] close")
	if c.hlsClient != nil {
		c.hlsClient.Close()
	}

	if c.mp4Client != nil {
		c.mp4Client.Close()
	}
}

func (c *Client) CodecData() ([]stream.Codec, error) {
	if c.hlsClient != nil {
		return c.hlsClient.CodecData()
	}

	if c.mp4Client != nil {
		return c.mp4Client.CodecData()
	}

	return nil, errors.New("not supported")
}

func (c *Client) PacketQueue() <-chan *stream.Packet {
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
