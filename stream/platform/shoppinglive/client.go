package shoppinglive

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jaesung9507/playgo/stream/protocol/hls"

	"github.com/deepch/vdk/av"
	"github.com/jaesung9507/nvver/shoppinglive"
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
	client := shoppinglive.NewClient(&http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	})

	var hlsURL *url.URL
	if broadcastID, ok := strings.CutPrefix(c.url.Path, "/lives/"); ok {
		broadcastID, err := strconv.ParseInt(broadcastID, 10, 64)
		if err != nil {
			return err
		}

		playback, err := client.GetLivePlayback(broadcastID)
		if err != nil {
			return err
		}

		rawURL := playback.GetHLSPath()
		if len(rawURL) <= 0 {
			return fmt.Errorf("not found hls path: %v", playback)
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
