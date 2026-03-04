package naver

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jaesung9507/playgo/stream/protocol/hls"
	httpStream "github.com/jaesung9507/playgo/stream/protocol/http"

	"github.com/deepch/vdk/av"
	"github.com/jaesung9507/nvver/chzzk"
	"github.com/jaesung9507/nvver/shoppinglive"
	"github.com/jaesung9507/nvver/tv"
	"github.com/jaesung9507/nvver/webtoon"
)

type Client struct {
	url        *url.URL
	hlsClient  *hls.Client
	httpClient *httpStream.HTTPClient
}

func New(parsedURL *url.URL) *Client {
	return &Client{
		url: parsedURL,
	}
}

func (c *Client) Dial() error {
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	var hlsURL, mp4URL *url.URL
	switch c.url.Host {
	case "chzzk.naver.com":
		client := chzzk.NewClient(httpClient)
		if channelID, ok := strings.CutPrefix(c.url.Path, "/live/"); ok {
			liveDetail, err := client.GetLiveDetail(channelID)
			if err != nil {
				return err
			}

			playback, err := liveDetail.GetLivePlayback()
			if err != nil {
				return err
			}

			rawURL := playback.HLSPath()
			if len(rawURL) <= 0 {
				return fmt.Errorf("status: %s", liveDetail.Status)
			}

			hlsURL, err = url.Parse(rawURL)
			if err != nil {
				return err
			}
		} else if videoNo, ok := strings.CutPrefix(c.url.Path, "/video/"); ok {
			videoNo, err := strconv.ParseInt(videoNo, 10, 64)
			if err != nil {
				return err
			}

			video, err := client.GetVideoInfo(videoNo)
			if err != nil {
				return err
			}

			if playback, err := video.GetLiveRewindPlayback(); err != nil {
				vods, err := client.GetVideoURL(video.VideoNo, video.VideoID, video.InKey)
				if err != nil {
					return err
				}

				resolution := 0
				for _, info := range vods {
					currentResolution := info.Width * info.Height
					if resolution < currentResolution {
						parsedURL, err := url.Parse(info.URL)
						if err != nil {
							continue
						}

						if ext := filepath.Ext(path.Base(parsedURL.Path)); ext == ".m3u8" {
							hlsURL = parsedURL
							resolution = currentResolution
						}
					}
				}
			} else {
				if rawURL := playback.HLSPath(); len(rawURL) > 0 {
					hlsURL, err = url.Parse(rawURL)
					if err != nil {
						return err
					}
				}
			}
		} else if clipID, ok := strings.CutPrefix(c.url.Path, "/clips/"); ok {
			clipDetail, err := client.GetClipDetail(clipID)
			if err != nil {
				return err
			}

			mp4URLs, err := client.GetClipMP4URL(clipDetail.ClipUID, clipDetail.VideoID)
			if err != nil {
				return err
			}

			for _, rawURL := range mp4URLs {
				mp4URL, err = url.Parse(rawURL)
				if err != nil {
					return err
				}
				break
			}
		}
	case "tv.naver.com":
		client := tv.NewClient(httpClient)
		if liveNo, ok := strings.CutPrefix(c.url.Path, "/l/"); ok {
			liveNo, err := strconv.ParseInt(liveNo, 10, 64)
			if err != nil {
				return err
			}

			playback, err := client.GetLivePlayback(liveNo)
			if err != nil {
				return err
			}

			rawURL := playback.HLSPath()
			if len(rawURL) <= 0 {
				return fmt.Errorf("not found hls path: %v", playback)
			}

			hlsURL, err = url.Parse(rawURL)
			if err != nil {
				return err
			}
		} else if vodNo, ok := strings.CutPrefix(c.url.Path, "/v/"); ok {
			vodNo, err := strconv.ParseInt(vodNo, 10, 64)
			if err != nil {
				return err
			}

			vodInfo, err := client.GetVODInfo(vodNo)
			if err != nil {
				return err
			}

			vods, err := client.GetVODURL(vodInfo.Clip.ClipNo, vodInfo.Clip.VideoID, vodInfo.Play.InKey)
			if err != nil {
				return err
			}

			resolution := 0
			for _, info := range vods {
				currentResolution := info.Width * info.Height
				if resolution < currentResolution {
					parsedURL, err := url.Parse(info.URL)
					if err != nil {
						continue
					}

					if ext := filepath.Ext(path.Base(parsedURL.Path)); ext == ".m3u8" {
						hlsURL = parsedURL
						resolution = currentResolution
					}
				}
			}
		} else if clipNo, ok := strings.CutPrefix(c.url.Path, "/h/"); ok {
			clipNo, err := strconv.ParseInt(clipNo, 10, 64)
			if err != nil {
				return err
			}

			videoID, err := client.GetClipVideoID(clipNo)
			if err != nil {
				return err
			}

			mp4URLs, err := client.GetClipMP4URL(videoID)
			if err != nil {
				return err
			}

			for _, rawURL := range mp4URLs {
				mp4URL, err = url.Parse(rawURL)
				if err != nil {
					return err
				}
				break
			}
		}
	case "view.shoppinglive.naver.com":
		client := shoppinglive.NewClient(httpClient)
		if broadcastID, ok := strings.CutPrefix(c.url.Path, "/lives/"); ok {
			broadcastID, err := strconv.ParseInt(broadcastID, 10, 64)
			if err != nil {
				return err
			}

			playback, err := client.GetLivePlayback(broadcastID)
			if err != nil {
				return err
			}

			rawURL := playback.HLSPath()
			if len(rawURL) <= 0 {
				return fmt.Errorf("not found hls path: %v", playback)
			}

			hlsURL, err = url.Parse(rawURL)
			if err != nil {
				return err
			}
		} else if shortClipID, ok := strings.CutPrefix(c.url.Path, "/shortclips/"); ok {
			shortClipID, err := strconv.ParseInt(shortClipID, 10, 64)
			if err != nil {
				return err
			}

			clip, err := client.GetShortClipInfo(shortClipID)
			if err != nil {
				return err
			}

			vods, err := client.GetShortClipURL(clip.ShortClipID, clip.VODMediaURL)
			if err != nil {
				return err
			}

			resolution := 0
			for _, info := range vods {
				currentResolution := info.Width * info.Height
				if resolution < currentResolution {
					parsedURL, err := url.Parse(info.URL)
					if err != nil {
						continue
					}

					if ext := filepath.Ext(path.Base(parsedURL.Path)); ext == ".m3u8" {
						if !strings.Contains(parsedURL.Path, "/cmaf/") {
							hlsURL = parsedURL
							resolution = currentResolution
						}
					}
				}
			}
		}
	case "comic.naver.com":
		httpClient.Jar, _ = cookiejar.New(nil)
		client := webtoon.NewClient(httpClient)
		if strings.HasPrefix(c.url.Path, "/cuts/") {
			cutsID := c.url.Query().Get("id")
			if len(cutsID) <= 0 {
				return errors.New("not found cuts id")
			}

			token, err := client.GetCutsToken(cutsID)
			if err != nil {
				return err
			}

			cuts, err := client.GetCutsInfo(cutsID)
			if err != nil {
				return err
			}

			vods, err := client.GetCutsURL(cutsID, cuts.AssetID(), token)
			if err != nil {
				return err
			}

			for _, info := range vods {
				parsedURL, err := url.Parse(info.URL)
				if err != nil {
					continue
				}

				if ext := filepath.Ext(path.Base(parsedURL.Path)); ext == ".mp4" {
					mp4URL = parsedURL
					break
				}
			}
		}
	}

	if hlsURL != nil {
		c.hlsClient = hls.New(hlsURL)
		return c.hlsClient.Dial()
	} else if mp4URL != nil {
		c.httpClient = httpStream.New(mp4URL)
		return c.httpClient.Dial()
	}

	return errors.New("not supported url")
}

func (c *Client) Close() {
	if c.hlsClient != nil {
		c.hlsClient.Close()
	}

	if c.httpClient != nil {
		c.httpClient.Close()
	}
}

func (c *Client) CodecData() ([]av.CodecData, error) {
	if c.hlsClient != nil {
		return c.hlsClient.CodecData()
	}

	if c.httpClient != nil {
		return c.httpClient.CodecData()
	}

	return nil, errors.New("not supported")
}

func (c *Client) PacketQueue() <-chan *av.Packet {
	if c.hlsClient != nil {
		return c.hlsClient.PacketQueue()
	}

	if c.httpClient != nil {
		return c.httpClient.PacketQueue()
	}

	return nil
}

func (c *Client) CloseCh() <-chan any {
	if c.hlsClient != nil {
		return c.hlsClient.CloseCh()
	}

	if c.httpClient != nil {
		return c.httpClient.CloseCh()
	}

	return nil
}
