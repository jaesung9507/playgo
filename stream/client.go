package stream

import (
	"fmt"
	"net/url"
	"path"
	"path/filepath"

	"github.com/jaesung9507/playgo/stream/hls"
	"github.com/jaesung9507/playgo/stream/http"
	"github.com/jaesung9507/playgo/stream/rtmp"
	"github.com/jaesung9507/playgo/stream/rtsp"
	"github.com/jaesung9507/playgo/stream/srt"

	"github.com/deepch/vdk/av"
)

type Client interface {
	Dial() error
	Close()
	CodecData() ([]av.CodecData, error)
	PacketQueue() <-chan *av.Packet
	CloseCh() <-chan any
}

func Dial(streamUrl string) (Client, error) {
	parsedUrl, err := url.Parse(streamUrl)
	if err != nil {
		return nil, err
	}

	var client Client
	switch parsedUrl.Scheme {
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

	if err = client.Dial(); err != nil {
		return nil, err
	}

	return client, nil
}
