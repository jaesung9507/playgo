package stream

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"path/filepath"

	"github.com/jaesung9507/playgo/stream/format"
	"github.com/jaesung9507/playgo/stream/platform/cime"
	"github.com/jaesung9507/playgo/stream/platform/naver"
	"github.com/jaesung9507/playgo/stream/platform/popkontv"
	"github.com/jaesung9507/playgo/stream/platform/sbs"
	"github.com/jaesung9507/playgo/stream/platform/tiktok"
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
	Secure() (bool, bool, map[string]string)
}

func Dial(ctx context.Context, streamURL string) (Client, error) {
	parsedURL, err := url.Parse(streamURL)
	if err != nil {
		return nil, err
	}

	var client Client
	switch parsedURL.Scheme {
	case "file":
		client = format.NewLocalFile(parsedURL.Path)
	case "rtsp", "rtsps":
		client = rtsp.New(parsedURL)
	case "rtmp", "rtmps":
		client = rtmp.New(parsedURL)
	case "http", "https":
		switch parsedURL.Host {
		case "ci.me":
			client = cime.New(parsedURL)
		case "popkontv.com", "www.popkontv.com":
			client = popkontv.New(parsedURL)
		case "sbs.co.kr", "www.sbs.co.kr", "allvod.sbs.co.kr", "programs.sbs.co.kr":
			client = sbs.New(parsedURL)
		case "tiktok.com", "www.tiktok.com":
			client = tiktok.New(parsedURL)
		case "chzzk.naver.com", "tv.naver.com", "view.shoppinglive.naver.com", "comic.naver.com":
			client = naver.New(parsedURL)
		case "youtube.com", "www.youtube.com", "music.youtube.com", "youtu.be", "youtubekids.com", "www.youtubekids.com":
			client = youtube.New(parsedURL)
		default:
			switch filepath.Ext(path.Base(parsedURL.Path)) {
			case ".m3u8":
				client = hls.New(parsedURL)
			case ".mp4":
				client = http.NewMP4Client(parsedURL)
			default:
				client = http.New(parsedURL)
			}
		}
	case "srt":
		client = srt.New(parsedURL)
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", parsedURL.Scheme)
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
			client.Close()
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
