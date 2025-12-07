package chzzk

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/jaesung9507/playgo/stream/protocol/hls"

	"github.com/deepch/vdk/av"
	"github.com/jaesung9507/chzzk"
)

type Client struct {
	url       *url.URL
	hlsClient *hls.HLSClient
}

func New(parsedURL *url.URL) *Client {
	return &Client{
		url: parsedURL,
	}
}

func (c *Client) Dial() error {
	client := chzzk.NewClient(&http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	})

	var hlsURL *url.URL
	if channelID, ok := strings.CutPrefix(c.url.Path, "/live/"); ok {
		liveDetail, err := client.GetLiveDetail(channelID)
		if err != nil {
			return err
		}

		playback, err := liveDetail.GetLivePlayback()
		if err != nil {
			return err
		}

		rawURL := playback.GetHLSPath()
		if len(rawURL) <= 0 {
			return fmt.Errorf("status: %s", liveDetail.Content.Status)
		}

		hlsURL, err = url.Parse(rawURL)
		if err != nil {
			return err
		}
	}

	if hlsURL == nil {
		return errors.New("not supported url")
	}

	c.hlsClient = hls.New(hlsURL)

	return c.hlsClient.Dial()
}

func (c *Client) Close() {
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
