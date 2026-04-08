package http

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/format/mp4"
)

type MP4Client struct {
	url         *url.URL
	closer      io.Closer
	demuxer     av.Demuxer
	signal      chan any
	packetQueue chan *av.Packet
}

func NewMP4Client(parsedUrl *url.URL) *MP4Client {
	return &MP4Client{
		url:         parsedUrl,
		signal:      make(chan any, 1),
		packetQueue: make(chan *av.Packet),
	}
}

func (c *MP4Client) DialWithHTTPClient(client *http.Client) error {
	return c.dial(client)
}

func (c *MP4Client) Dial() error {
	return c.dial(&http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	})
}

func (c *MP4Client) dial(client *http.Client) error {
	log.Printf("[HTTP-MP4] dial: %s", c.url.String())
	srs, err := NewStreamingReadSeeker(c.url.String(), client)
	if err != nil {
		log.Printf("[HTTP-MP4] failed to mp4 streaming: %v", err)
		resp, err := client.Get(c.url.String())
		if err != nil {
			return err
		}
		c.closer = resp.Body

		if resp.StatusCode != http.StatusOK {
			c.Close()
			return fmt.Errorf("status code: %s", resp.Status)
		}

		contentLength, _ := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
		if contentLength <= 0 {
			c.Close()
			return fmt.Errorf("not supported for live streams: %d", contentLength)
		}
		log.Printf("[HTTP-MP4] Content-Length: %d", contentLength)

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Close()
			return err
		}
		log.Print("[HTTP-MP4] finish download")

		c.demuxer = mp4.NewDemuxer(bytes.NewReader(data))
	} else {
		c.demuxer = mp4.NewDemuxer(srs)
		c.closer = srs
	}

	return nil
}

func (c *MP4Client) Close() {
	log.Print("[HTTP-MP4] close")
	if c.closer != nil {
		c.closer.Close()
	}
}

func (c *MP4Client) CodecData() ([]av.CodecData, error) {
	streams, err := c.demuxer.Streams()
	if err == nil {
		go func() {
			var baseCtsOffset time.Duration
			for {
				packet, err := c.demuxer.ReadPacket()
				if err != nil {
					log.Printf("[HTTP-MP4] finish: %v", err)
					if !errors.Is(err, io.EOF) {
						c.signal <- err
					}
					return
				}

				// WORKAROUND: Correct invalid cts values from the vdk mp4 demuxer that cause decode failures on macOS
				if packet.IsKeyFrame && packet.CompositionTime > 0 {
					baseCtsOffset = packet.CompositionTime
				} else if baseCtsOffset > 0 && packet.CompositionTime == 0 {
					baseCtsOffset = 0
				}
				if baseCtsOffset > 0 {
					packet.CompositionTime -= baseCtsOffset
				}

				c.packetQueue <- &packet
			}
		}()
	}
	return streams, err
}

func (c *MP4Client) PacketQueue() <-chan *av.Packet {
	return c.packetQueue
}

func (c *MP4Client) CloseCh() <-chan any {
	return c.signal
}
