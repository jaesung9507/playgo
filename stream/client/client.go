package client

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"path/filepath"

	"github.com/jaesung9507/playgo/stream"
	"github.com/jaesung9507/playgo/stream/format"
	"github.com/jaesung9507/playgo/stream/platform/cime"
	"github.com/jaesung9507/playgo/stream/platform/naver"
	"github.com/jaesung9507/playgo/stream/platform/pandatv"
	"github.com/jaesung9507/playgo/stream/platform/popkontv"
	"github.com/jaesung9507/playgo/stream/platform/sbs"
	"github.com/jaesung9507/playgo/stream/platform/tiktok"
	"github.com/jaesung9507/playgo/stream/platform/youtube"
	"github.com/jaesung9507/playgo/stream/protocol/hls"
	"github.com/jaesung9507/playgo/stream/protocol/http"
	"github.com/jaesung9507/playgo/stream/protocol/rtmp"
	"github.com/jaesung9507/playgo/stream/protocol/rtsp"
	"github.com/jaesung9507/playgo/stream/protocol/srt"
)

func Dial(ctx context.Context, streamURL string) (stream.Client, error) {
	parsedURL, err := url.Parse(streamURL)
	if err != nil {
		return nil, err
	}

	var c stream.Client
	switch parsedURL.Scheme {
	case "file":
		c = format.NewLocalFile(parsedURL.Path)
	case "rtsp", "rtsps":
		c = rtsp.New(parsedURL)
	case "rtmp", "rtmps":
		c = rtmp.New(parsedURL)
	case "http", "https":
		switch parsedURL.Host {
		case "ci.me":
			c = cime.New(parsedURL)
		case "pandalive.co.kr", "www.pandalive.co.kr":
			c = pandatv.New(parsedURL)
		case "popkontv.com", "www.popkontv.com":
			c = popkontv.New(parsedURL)
		case "sbs.co.kr", "www.sbs.co.kr", "allvod.sbs.co.kr", "programs.sbs.co.kr":
			c = sbs.New(parsedURL)
		case "tiktok.com", "www.tiktok.com":
			c = tiktok.New(parsedURL)
		case "chzzk.naver.com", "tv.naver.com", "view.shoppinglive.naver.com", "comic.naver.com":
			c = naver.New(parsedURL)
		case "youtube.com", "www.youtube.com", "music.youtube.com", "youtu.be", "youtubekids.com", "www.youtubekids.com":
			c = youtube.New(parsedURL)
		default:
			switch filepath.Ext(path.Base(parsedURL.Path)) {
			case ".m3u8":
				c = hls.New(parsedURL)
			case ".mp4":
				c = http.NewMP4Client(parsedURL)
			default:
				c = http.New(parsedURL)
			}
		}
	case "srt":
		c = srt.New(parsedURL)
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", parsedURL.Scheme)
	}

	ch := make(chan error, 1)
	go func() {
		ch <- c.Dial()
	}()

	select {
	case <-ctx.Done():
		go func() {
			<-ch
			c.Close()
		}()
		return nil, context.Canceled
	case err := <-ch:
		if err != nil {
			c.Close()
			return nil, err
		}
	}

	return c, nil
}

func CodecData(ctx context.Context, c stream.Client) (codecs []stream.Codec, err error) {
	defer func() {
		if err != nil {
			c.Close()
		}
	}()

	ch := make(chan error, 1)
	go func() {
		var err error
		codecs, err = c.CodecData()
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
