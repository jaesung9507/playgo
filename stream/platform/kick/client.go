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
	client := (&secure.TLS{}).HTTPClient()
	log.Printf("[KICK] dial: %s", c.url.String())
	var hlsURL *url.URL
	path := strings.Split(c.url.Path, "/")
	if len(path) > 3 && path[2] == "videos" {
		rawURL, err := GetVideoHLSURL(client, path[3])
		if err != nil {
			return err
		}

		hlsURL, err = url.Parse(rawURL)
		if err != nil {
			return err
		}
	} else if len(path) > 3 && path[2] == "clips" {
		clip, err := GetClip(client, path[3])
		if err != nil {
			return err
		}
		log.Printf("[KICK] video title: %s", clip.Title)

		hlsURL, err = url.Parse(clip.ClipURL)
		if err != nil {
			return err
		}
	} else if len(path) > 1 {
		video, err := GetLiveStream(client, path[1])
		if err != nil {
			return err
		}
		log.Printf("[KICK] video title: %s", video.Title)

		hlsURL, err = url.Parse(video.PlaybackURL)
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
