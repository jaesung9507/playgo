package http

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
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

type HTTPClient struct {
	url         *url.URL
	closer      io.Closer
	demuxer     av.Demuxer
	signal      chan any
	packetQueue chan *av.Packet
	isLive      bool
}

func New(parsedUrl *url.URL) *HTTPClient {
	return &HTTPClient{
		url:         parsedUrl,
		signal:      make(chan any, 1),
		packetQueue: make(chan *av.Packet),
	}
}

func (h *HTTPClient) getDemuxerFunc() (func(r io.Reader) (av.Demuxer, error), error) {
	ext := filepath.Ext(path.Base(h.url.Path))
	switch ext {
	case ".flv":
		return func(r io.Reader) (av.Demuxer, error) { return flv.NewDemuxer(r), nil }, nil
	case ".ts":
		return func(r io.Reader) (av.Demuxer, error) { return ts.NewDemuxer(r), nil }, nil
	case ".mp4":
		return func(r io.Reader) (av.Demuxer, error) {
			if h.isLive {
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

func (h *HTTPClient) Dial() error {
	newDemuxer, err := h.getDemuxerFunc()
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
	resp, err := client.Get(h.url.String())
	if err != nil {
		return err
	}
	h.closer = resp.Body

	contentLength, _ := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	if contentLength <= 0 {
		h.isLive = true
	}

	if resp.StatusCode != http.StatusOK {
		h.Close()
		return fmt.Errorf("status code: %s", resp.Status)
	}

	if h.demuxer, err = newDemuxer(resp.Body); err != nil {
		h.Close()
		return err
	}

	return nil
}

func (h *HTTPClient) Close() {
	if h.closer != nil {
		h.closer.Close()
	}
}

func (h *HTTPClient) CodecData() ([]av.CodecData, error) {
	streams, err := h.demuxer.Streams()
	if err == nil {
		go func() {
			for {
				packet, err := h.demuxer.ReadPacket()
				if err != nil {
					if h.isLive || !errors.Is(err, io.EOF) {
						h.signal <- err
					}
					return
				}
				h.packetQueue <- &packet
			}
		}()
	}
	return streams, err
}

func (h *HTTPClient) PacketQueue() <-chan *av.Packet {
	return h.packetQueue
}

func (h *HTTPClient) CloseCh() <-chan any {
	return h.signal
}
