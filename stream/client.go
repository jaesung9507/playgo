package stream

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"path/filepath"

	"github.com/jaesung9507/playgo/stream/file"
	"github.com/jaesung9507/playgo/stream/platform/youtube"
	"github.com/jaesung9507/playgo/stream/protocol/hls"
	"github.com/jaesung9507/playgo/stream/protocol/http"
	"github.com/jaesung9507/playgo/stream/protocol/rtmp"
	"github.com/jaesung9507/playgo/stream/protocol/rtsp"
	"github.com/jaesung9507/playgo/stream/protocol/srt"

	"github.com/deepch/vdk/av"
)

type Client interface {
	Dial() error
	Close()
	CodecData() ([]av.CodecData, error)
	PacketQueue() <-chan *av.Packet
	CloseCh() <-chan any
}

func Dial(ctx context.Context, streamUrl string) (Client, error) {
	parsedUrl, err := url.Parse(streamUrl)
	if err != nil {
		return nil, err
	}

	var client Client
	switch parsedUrl.Host {
	case "www.youtube.com", "youtu.be":
		client = youtube.New(streamUrl)
	default:
		switch parsedUrl.Scheme {
		case "file":
			client = file.New(parsedUrl.Path)
		case "rtsp", "rtsps":
			client = rtsp.New(parsedUrl)
		case "rtmp", "rtmps":
			client = rtmp.New(parsedUrl)
		case "http", "https":
			switch filepath.Ext(path.Base(parsedUrl.Path)) {
			case ".m3u8":
				client = hls.New(parsedUrl)
			default:
				client = http.New(parsedUrl)
			}
		case "srt":
			client = srt.New(parsedUrl)
		default:
			return nil, fmt.Errorf("unsupported protocol: %s", parsedUrl.Scheme)
		}
	}

	ch := make(chan error, 1)
	go func() {
		ch <- client.Dial()
	}()

	select {
	case <-ctx.Done():
		go func() {
			<-ch
			client.Close()
		}()
		return nil, context.Canceled
	case err := <-ch:
		if err != nil {
			return nil, err
		}
	}

	return client, nil
}

func CodecData(ctx context.Context, client Client) (codecs []av.CodecData, err error) {
	defer func() {
		if err != nil {
			client.Close()
		}
	}()

	ch := make(chan error, 1)
	go func() {
		var err error
		codecs, err = client.CodecData()
		ch <- err
	}()

	select {
	case <-ctx.Done():
		return nil, context.Canceled
	case err := <-ch:
		if err != nil {
			return nil, err
		}
	}

	return codecs, nil
}
