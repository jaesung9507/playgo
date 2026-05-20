package route

import (
	"fmt"
	"net/url"

	"github.com/jaesung9507/playgo/stream"
	"github.com/jaesung9507/playgo/stream/protocol/rtmp"
	"github.com/jaesung9507/playgo/stream/protocol/rtsp"
	"github.com/jaesung9507/playgo/stream/protocol/srt"
)

func NewClient(streamURL string) (stream.Client, error) {
	parsedURL, err := url.Parse(streamURL)
	if err != nil {
		return nil, err
	}

	switch parsedURL.Scheme {
	case "file":
		return NewLocalFile(parsedURL.Path)
	case "rtsp", "rtsps":
		return rtsp.New(parsedURL), nil
	case "rtmp", "rtmps":
		return rtmp.New(parsedURL), nil
	case "http", "https":
		return NewHTTPClient(parsedURL)
	case "srt":
		return srt.New(parsedURL), nil
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", parsedURL.Scheme)
	}
}
