package http

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/jaesung9507/playgo/secure"
	"github.com/jaesung9507/playgo/stream"
	"github.com/jaesung9507/playgo/stream/vdk"

	"github.com/deepch/vdk/format/mp4"
)

type NewDemuxerFunc func(io.ReadSeeker) stream.Demuxer

type MP4Client struct {
	url         *url.URL
	closer      io.Closer
	demuxer     stream.Demuxer
	signal      chan any
	packetQueue chan *stream.Packet
	tls         secure.TLS
	httpClient  *http.Client
	newDemuxer  NewDemuxerFunc
	workaround  bool
}

func NewMP4Client(parsedUrl *url.URL) *MP4Client {
	return &MP4Client{
		url:         parsedUrl,
		signal:      make(chan any, 1),
		packetQueue: make(chan *stream.Packet),
		newDemuxer: func(r io.ReadSeeker) stream.Demuxer {
			return vdk.ToDemuxer(mp4.NewDemuxer(r))
		},
		workaround: true,
	}
}

func (c *MP4Client) SetHTTPClient(client *http.Client) {
	c.httpClient = client
}

func (c *MP4Client) SetNewDemuxer(fn NewDemuxerFunc) {
	c.newDemuxer = fn
	c.workaround = false
}

func (c *MP4Client) Dial() error {
	if c.httpClient == nil {
		c.httpClient = c.tls.HTTPClient()
	}

	log.Printf("[HTTP-MP4] dial: %s", c.url.String())
	srs, err := NewStreamingReadSeeker(c.url.String(), c.httpClient)
	if err != nil {
		log.Printf("[HTTP-MP4] failed to mp4 streaming: %v", err)
		resp, err := c.httpClient.Get(c.url.String())
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

		c.demuxer = c.newDemuxer(bytes.NewReader(data))
	} else {
		c.demuxer = c.newDemuxer(srs)
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

func (c *MP4Client) CodecData() ([]stream.Codec, error) {
	codecs, err := c.demuxer.CodecData()
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

				if c.workaround {
					// WORKAROUND: Correct invalid cts values from the vdk mp4 demuxer that cause decode failures on macOS
					if packet.IsKeyFrame && packet.CompositionTime > 0 {
						baseCtsOffset = packet.CompositionTime
					} else if baseCtsOffset > 0 && packet.CompositionTime == 0 {
						baseCtsOffset = 0
					}
					if baseCtsOffset > 0 {
						packet.CompositionTime -= baseCtsOffset
					}
				}

				c.packetQueue <- &packet
			}
		}()
	}

	return codecs, err
}

func (c *MP4Client) PacketQueue() <-chan *stream.Packet {
	return c.packetQueue
}

func (c *MP4Client) CloseCh() <-chan any {
	return c.signal
}

func (c *MP4Client) Secure() (bool, bool, map[string]string) {
	return c.tls.Info()
}
