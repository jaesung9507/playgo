package youtube

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/jaesung9507/playgo/stream/protocol/hls"
	httpStream "github.com/jaesung9507/playgo/stream/protocol/http"

	"github.com/deepch/vdk/av"
	"github.com/kkdai/youtube/v2"
)

type Client struct {
	url       *url.URL
	hlsClient *hls.Client
	mp4Client *httpStream.MP4Client
}

func New(parsedURL *url.URL) *Client {
	return &Client{url: parsedURL}
}

func (c *Client) Dial() error {
	client := youtube.Client{
		HTTPClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		},
	}

	log.Printf("[YouTube] dial: %s", c.url.String())
	youtubeURL := c.url.String()
	if videoID, ok := strings.CutPrefix(c.url.Path, "/live/"); ok {
		youtubeURL = fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)
	}

	video, err := client.GetVideo(youtubeURL)
	if err != nil {
		return err
	}
	log.Printf("[YouTube] video title: %s", video.Title)

	if len(video.HLSManifestURL) > 0 {
		hlsURL, err := url.Parse(video.HLSManifestURL)
		if err != nil {
			return err
		}
		c.hlsClient = hls.New(hlsURL)

		return c.hlsClient.Dial()
	}

	formats := video.Formats.WithAudioChannels().Type("video/mp4")
	if len(formats) == 0 {
		return errors.New("not found mp4 formats")
	}
	log.Printf("[Youtube] quality: %s", formats[0].QualityLabel)

	streamURL, err := client.GetStreamURL(video, &formats[0])
	if err != nil {
		return err
	}

	mp4URL, err := url.Parse(streamURL)
	if err != nil {
		return err
	}
	c.mp4Client = httpStream.NewMP4Client(mp4URL)

	return c.mp4Client.Dial()
}

func (c *Client) Close() {
	log.Print("[YouTube] close")
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
