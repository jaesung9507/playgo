package youtube

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/jaesung9507/playgo/stream/protocol/hls"

	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/format/mp4"
	"github.com/kkdai/youtube/v2"
)

type Client struct {
	url         *url.URL
	demuxer     *mp4.Demuxer
	signal      chan any
	packetQueue chan *av.Packet

	hlsClient *hls.HLSClient
}

func New(parsedURL *url.URL) *Client {
	return &Client{
		url:         parsedURL,
		signal:      make(chan any, 1),
		packetQueue: make(chan *av.Packet),
	}
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

	youtubeURL := c.url.String()
	if strings.HasPrefix(c.url.Path, "/live/") {
		videoID := c.url.Path[6:]
		youtubeURL = fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)
	}

	video, err := client.GetVideo(youtubeURL)
	if err != nil {
		return err
	}

	if len(video.HLSManifestURL) > 0 {
		hlsURL, err := url.Parse(video.HLSManifestURL)
		if err != nil {
			return err
		}

		c.hlsClient = hls.New(hlsURL)
		return c.hlsClient.Dial()
	}

	formats := video.Formats.WithAudioChannels()
	stream, _, err := client.GetStream(video, &formats[0])
	if err != nil {
		return err
	}
	defer stream.Close()

	data, err := io.ReadAll(stream)
	if err != nil {
		return err
	}

	c.demuxer = mp4.NewDemuxer(bytes.NewReader(data))

	return nil
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

	streams, err := c.demuxer.Streams()
	if err == nil {
		go func() {
			for {
				packet, err := c.demuxer.ReadPacket()
				if err != nil {
					if !errors.Is(err, io.EOF) {
						c.signal <- err
					}
					return
				}
				c.packetQueue <- &packet
			}
		}()
	}
	return streams, err
}

func (c *Client) PacketQueue() <-chan *av.Packet {
	if c.hlsClient != nil {
		return c.hlsClient.PacketQueue()
	}
	return c.packetQueue
}

func (c *Client) CloseCh() <-chan any {
	if c.hlsClient != nil {
		return c.hlsClient.CloseCh()
	}
	return c.signal
}
