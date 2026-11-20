package tiktok

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
)

const (
	StatusOK      = 0
	StatusOffline = 4
)

type transport struct {
	Transport http.RoundTripper
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Add("Referer", "https://www.tiktok.com/")
	return t.Transport.RoundTrip(req)
}

func GetLiveFLVURL(client *http.Client, uniqueID string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://www.tiktok.com/@%s/live", uniqueID), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/157.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read body: %w", err)
	}

	m := regexp.MustCompile(`(?s)<script [^>]*id="SIGI_STATE"[^>]*>(.*?)</script>`).FindSubmatch(data)
	if len(m) < 2 {
		return "", fmt.Errorf("not found data: content-length=%d", len(data))
	}

	result := &struct {
		Data struct {
			Info struct {
				LiveRoom struct {
					Status     int `json:"status"`
					StreamData struct {
						PullData struct {
							StreamData string `json:"stream_data"`
						} `json:"pull_data"`
					} `json:"streamData"`
				} `json:"liveRoom"`
			} `json:"liveRoomUserInfo"`
		} `json:"LiveRoom"`
	}{}
	if err = json.Unmarshal(m[1], result); err != nil {
		return "", fmt.Errorf("failed to unmarshal data: %w", err)
	}

	if result.Data.Info.LiveRoom.Status == StatusOffline {
		return "", errors.New("channel is offline")
	}

	streamData := &struct {
		Data map[string]struct {
			Main struct {
				FLV string `json:"flv"`
			} `json:"main"`
		} `json:"data"`
	}{}
	if err = json.Unmarshal([]byte(result.Data.Info.LiveRoom.StreamData.PullData.StreamData), streamData); err != nil {
		return "", err
	}

	if stream, ok := streamData.Data["origin"]; ok && len(stream.Main.FLV) > 0 {
		return stream.Main.FLV, nil
	}

	for key, stream := range streamData.Data {
		if key == "ao" && len(streamData.Data) > 1 {
			continue
		}

		if len(stream.Main.FLV) > 0 {
			return stream.Main.FLV, nil
		}
	}

	return "", fmt.Errorf("not found flv url: %+v", streamData)
}

func GetVideoMP4URL(client *http.Client, videoURL string) (string, error) {
	resp, err := client.Get(videoURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("http status code: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read body: %w", err)
	}

	m := regexp.MustCompile(`(?s)<script [^>]*id="__UNIVERSAL_DATA_FOR_REHYDRATION__"[^>]*>(.*?)</script>`).FindSubmatch(data)
	if len(m) < 2 {
		return "", fmt.Errorf("not found data: content-length=%d", len(data))
	}

	result := &struct {
		DefaultScope struct {
			VideoDetail struct {
				ItemInfo struct {
					ItemStruct struct {
						Video struct {
							DownloadAddr string `json:"downloadAddr"`
							PlayAddr     string `json:"playAddr"`
						} `json:"video"`
					} `json:"itemStruct"`
				} `json:"itemInfo"`
				StatusCode int    `json:"statusCode"`
				StatusMsg  string `json:"statusMsg"`
			} `json:"webapp.video-detail"`
		} `json:"__DEFAULT_SCOPE__"`
	}{}
	if err = json.Unmarshal(m[1], result); err != nil {
		return "", fmt.Errorf("failed to unmarshal data: %w", err)
	}

	detail := result.DefaultScope.VideoDetail
	if detail.StatusCode != StatusOK {
		return "", fmt.Errorf("api status code: %d, msg: %s", detail.StatusCode, detail.StatusMsg)
	}

	mp4URL := detail.ItemInfo.ItemStruct.Video.DownloadAddr
	if len(videoURL) <= 0 {
		if len(detail.ItemInfo.ItemStruct.Video.PlayAddr) <= 0 {
			return "", fmt.Errorf("not found video url: %+v", result)
		}
		mp4URL = detail.ItemInfo.ItemStruct.Video.PlayAddr
	}

	return mp4URL, nil
}
