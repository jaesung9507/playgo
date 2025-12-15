package chzzk

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/jaesung9507/playgo/stream/protocol/hls"
	httpStream "github.com/jaesung9507/playgo/stream/protocol/http"

	"github.com/deepch/vdk/av"
	"github.com/jaesung9507/chzzk"
)

type Client struct {
	url        *url.URL
	hlsClient  *hls.HLSClient
	httpClient *httpStream.HTTPClient
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

	var hlsURL, mp4URL *url.URL
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
	} else if clipID, ok := strings.CutPrefix(c.url.Path, "/clips/"); ok {
		clipDetail, err := client.GetClipDetail(clipID)
		if err != nil {
			return err
		}

		mp4URLs, err := client.GetClipMP4URL(clipDetail.Content.ClipUID, clipDetail.Content.VideoID)
		if err != nil {
			return err
		}

		for _, rawURL := range mp4URLs {
			mp4URL, err = url.Parse(rawURL)
			if err != nil {
				return err
			}
			break
		}
	}

	if hlsURL != nil {
		c.hlsClient = hls.New(hlsURL)
		return c.hlsClient.Dial()
	} else if mp4URL != nil {
		c.httpClient = httpStream.New(mp4URL)
		return c.httpClient.Dial()
	}

	return errors.New("not supported url")
}

func (c *Client) Close() {
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
