package youtube

import (
	"bytes"
	"crypto/tls"
	"errors"
	"io"
	"net/http"

	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/format/mp4"
	"github.com/kkdai/youtube/v2"
)

type Client struct {
	url         string
	demuxer     *mp4.Demuxer
	signal      chan any
	packetQueue chan *av.Packet
}

func New(url string) *Client {
	return &Client{
		url:         url,
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

	video, err := client.GetVideo(c.url)
	if err != nil {
		return err
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
}

func (c *Client) CodecData() ([]av.CodecData, error) {
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
	return c.packetQueue
}

func (c *Client) CloseCh() <-chan any {
	return c.signal
}
