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
	"path"
	"path/filepath"
	"strconv"

	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/format/flv"
	"github.com/deepch/vdk/format/mp4"
	"github.com/deepch/vdk/format/ts"
)

type Client struct {
	url         *url.URL
	closer      io.Closer
	demuxer     av.Demuxer
	signal      chan any
	packetQueue chan *av.Packet
	isLive      bool
}

func New(parsedUrl *url.URL) *Client {
	return &Client{
		url:         parsedUrl,
		signal:      make(chan any, 1),
		packetQueue: make(chan *av.Packet),
	}
}

func (c *Client) getDemuxerFunc() (func(r io.Reader) (av.Demuxer, error), error) {
	ext := filepath.Ext(path.Base(c.url.Path))
	switch ext {
	case ".flv":
		return func(r io.Reader) (av.Demuxer, error) { return flv.NewDemuxer(r), nil }, nil
	case ".ts":
		return func(r io.Reader) (av.Demuxer, error) { return ts.NewDemuxer(r), nil }, nil
	case ".mp4":
		return func(r io.Reader) (av.Demuxer, error) {
			if c.isLive {
				return nil, fmt.Errorf("not supported for live streams: %s", ext)
			}
			data, err := io.ReadAll(r)
			if err != nil {
				return nil, err
			}
			return mp4.NewDemuxer(bytes.NewReader(data)), nil
		}, nil
	}
	return nil, fmt.Errorf("unsupported extension: %s", ext)
}

func (c *Client) Dial() error {
	log.Printf("[HTTP] dial: %s", c.url.String())
	newDemuxer, err := c.getDemuxerFunc()
	if err != nil {
		return err
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
	resp, err := client.Get(c.url.String())
	if err != nil {
		return err
	}
	c.closer = resp.Body

	contentLength, _ := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	if contentLength <= 0 {
		c.isLive = true
	}

	if resp.StatusCode != http.StatusOK {
		c.Close()
		return fmt.Errorf("status code: %s", resp.Status)
	}

	if c.demuxer, err = newDemuxer(resp.Body); err != nil {
		c.Close()
		return err
	}

	return nil
}

func (c *Client) Close() {
	log.Print("[HTTP] close")
	if c.closer != nil {
		c.closer.Close()
	}
}

func (c *Client) CodecData() ([]av.CodecData, error) {
	streams, err := c.demuxer.Streams()
	if err == nil {
		go func() {
			for {
				packet, err := c.demuxer.ReadPacket()
				if err != nil {
					if c.isLive || !errors.Is(err, io.EOF) {
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
