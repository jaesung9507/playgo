package route

import (
	"fmt"
	"io"
	"net/url"
	"path"
	"path/filepath"

	"github.com/deepch/vdk/format/flv"
	"github.com/jaesung9507/playgo/stream"
	"github.com/jaesung9507/playgo/stream/codec/h26x/h264"
	"github.com/jaesung9507/playgo/stream/codec/h26x/h265"
	"github.com/jaesung9507/playgo/stream/format/ts"
	"github.com/jaesung9507/playgo/stream/platform/cime"
	"github.com/jaesung9507/playgo/stream/platform/kick"
	"github.com/jaesung9507/playgo/stream/platform/naver"
	"github.com/jaesung9507/playgo/stream/platform/pandatv"
	"github.com/jaesung9507/playgo/stream/platform/popkontv"
	"github.com/jaesung9507/playgo/stream/platform/sbs"
	"github.com/jaesung9507/playgo/stream/platform/tiktok"
	"github.com/jaesung9507/playgo/stream/platform/youtube"
	"github.com/jaesung9507/playgo/stream/protocol/hls"
	"github.com/jaesung9507/playgo/stream/protocol/http"
	"github.com/jaesung9507/playgo/stream/protocol/whep"
	"github.com/jaesung9507/playgo/stream/vdk"
)

func getHTTPDemuxer(ext string) stream.GetNetworkDemuxerFunc {
	switch ext {
	case ".flv":
		return func(r io.Reader) (stream.Demuxer, error) { return vdk.ToDemuxer(flv.NewDemuxer(r)), nil }
	case ".ts":
		return func(r io.Reader) (stream.Demuxer, error) { return ts.NewDemuxer(r), nil }
	case ".h264", ".264":
		return func(r io.Reader) (stream.Demuxer, error) { return h264.NewDemuxer(r), nil }
	case ".h265", ".265", ".hevc":
		return func(r io.Reader) (stream.Demuxer, error) { return h265.NewDemuxer(r), nil }
	default:
		return nil
	}
}

func NewHTTPClient(parsedURL *url.URL) (stream.Client, error) {
	switch parsedURL.Host {
	case "ci.me":
		return cime.New(parsedURL), nil
	case "kick.com", "www.kick.com":
		return kick.New(parsedURL), nil
	case "pandalive.co.kr", "www.pandalive.co.kr":
		return pandatv.New(parsedURL), nil
	case "popkontv.com", "www.popkontv.com":
		return popkontv.New(parsedURL), nil
	case "sbs.co.kr", "www.sbs.co.kr", "allvod.sbs.co.kr", "programs.sbs.co.kr":
		return sbs.New(parsedURL), nil
	case "tiktok.com", "www.tiktok.com":
		return tiktok.New(parsedURL), nil
	case "chzzk.naver.com", "tv.naver.com", "view.shoppinglive.naver.com", "comic.naver.com":
		return naver.New(parsedURL), nil
	case "youtube.com", "www.youtube.com", "music.youtube.com", "youtu.be", "youtubekids.com", "www.youtubekids.com":
		return youtube.New(parsedURL), nil
	default:
		ext := filepath.Ext(path.Base(parsedURL.Path))
		switch ext {
		case ".m3u8":
			return hls.New(parsedURL), nil
		case ".mp4":
			return http.NewMP4Client(parsedURL), nil
		default:
			if fn := getHTTPDemuxer(ext); fn != nil {
				return http.New(parsedURL, fn), nil
			} else if CheckWHEP(parsedURL.String()) {
				return whep.New(parsedURL), nil
			}

			return nil, fmt.Errorf("unsupported http extension: %s", ext)
		}
	}
}
