package kick

import (
	"encoding/json"
	"fmt"
	"net/http"
)

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36"

type LiveStream struct {
	Title       string `json:"session_title"`
	PlaybackURL string `json:"playback_url"`
}

type Clip struct {
	Title   string `json:"title"`
	ClipURL string `json:"clip_url"`
}

func GetLiveStream(client *http.Client, channelSlug string) (*LiveStream, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://kick.com/api/v2/channels/%s/livestream", channelSlug), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	result := &struct {
		Data LiveStream `json:"data"`
	}{}
	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result.Data, nil
}

func GetVideoHLSURL(client *http.Client, uuid string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://kick.com/api/v1/video/%s", uuid), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	result := &struct {
		Src string `json:"source"`
	}{}
	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Src, nil
}

func GetClip(client *http.Client, clipID string) (*Clip, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://kick.com/api/v2/clips/%s", clipID), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	result := &struct {
		Clip Clip `json:"clip"`
	}{}
	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result.Clip, nil
}
