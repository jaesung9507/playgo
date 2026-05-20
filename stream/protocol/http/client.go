package http

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"

	"github.com/jaesung9507/playgo/secure"
	"github.com/jaesung9507/playgo/stream"
)

type Client struct {
	url         *url.URL
	closer      io.Closer
	demuxer     stream.Demuxer
	signal      chan any
	packetQueue chan *stream.Packet
	isLive      bool
	tls         secure.TLS
	getDemuxer  stream.GetNetworkDemuxerFunc
}

func New(parsedUrl *url.URL, getDemuxer stream.GetNetworkDemuxerFunc) *Client {
	return &Client{
		url:         parsedUrl,
		signal:      make(chan any, 1),
		packetQueue: make(chan *stream.Packet),
		getDemuxer:  getDemuxer,
	}
}

func (c *Client) Dial() error {
	log.Printf("[HTTP] dial: %s", c.url.String())
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: c.tls.Config(),
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

	if c.demuxer, err = c.getDemuxer(resp.Body); err != nil {
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

func (c *Client) CodecData() ([]stream.Codec, error) {
	codecs, err := c.demuxer.CodecData()
	if err == nil {
		go func() {
			for {
				packet, err := c.demuxer.ReadPacket()
				if err != nil {
					log.Printf("[HTTP] finish: %v", err)
					if c.isLive || !errors.Is(err, io.EOF) {
						c.signal <- err
					}
					return
				}
				c.packetQueue <- &packet
			}
		}()
	}

	return codecs, err
}

func (c *Client) PacketQueue() <-chan *stream.Packet {
	return c.packetQueue
}

func (c *Client) CloseCh() <-chan any {
	return c.signal
}

func (c *Client) Secure() (bool, bool, map[string]string) {
	return c.tls.Info()
}
