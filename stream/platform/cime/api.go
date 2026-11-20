package cime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"

	"github.com/dop251/goja"
)

type SignCookieReq struct {
	Type    string `json:"type"`
	ClipID  string `json:"clipId,omitempty"`
	VideoID string `json:"videoId,omitempty"`
}

func GetLiveHLSURL(client *http.Client, channelSlug string) (string, error) {
	resp, err := client.Get(fmt.Sprintf("https://ci.me/@%s/live", channelSlug))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read body: %w", err)
	}

	m := regexp.MustCompile(`(?s)JSON\.parse\((['"].+?['"])\)`).FindSubmatch(data)
	if len(m) < 2 {
		return "", fmt.Errorf("not found data: content-length=%d", len(data))
	}

	v, err := goja.New().RunString(string(m[1]))
	if err != nil {
		return "", fmt.Errorf("failed to parse raw json: %w", err)
	}

	result := &struct {
		Args []struct {
			BodyData struct {
				Live struct {
					Playback struct {
						URL string `json:"url"`
					} `json:"playback"`
				} `json:"live"`
			} `json:"bodyData"`
		} `json:"args"`
	}{}
	if err = json.Unmarshal([]byte(v.String()), result); err != nil {
		return "", fmt.Errorf("failed to unmarshal data: %w", err)
	}

	if len(result.Args) < 1 || len(result.Args[0].BodyData.Live.Playback.URL) <= 0 {
		return "", fmt.Errorf("not found playback url: %+v", result)
	}

	return result.Args[0].BodyData.Live.Playback.URL, nil
}

func GetVODHLSURL(client *http.Client, channelSlug, vodID string) (string, error) {
	resp, err := client.Get(fmt.Sprintf("https://ci.me/@%s/vods/%s", channelSlug, vodID))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read body: %w", err)
	}

	m := regexp.MustCompile(`(?s)JSON\.parse\((['"].+?['"])\)`).FindSubmatch(data)
	if len(m) < 2 {
		return "", fmt.Errorf("not found data: content-length=%d", len(data))
	}

	v, err := goja.New().RunString(string(m[1]))
	if err != nil {
		return "", fmt.Errorf("failed to parse raw json: %w", err)
	}

	result := &struct {
		Args []struct {
			BodyData struct {
				VOD struct {
					Playback struct {
						URL string `json:"url"`
					} `json:"playback"`
				} `json:"vod"`
			} `json:"bodyData"`
		} `json:"args"`
	}{}
	if err = json.Unmarshal([]byte(v.String()), result); err != nil {
		return "", fmt.Errorf("failed to unmarshal data: %w", err)
	}

	if len(result.Args) < 1 || len(result.Args[0].BodyData.VOD.Playback.URL) <= 0 {
		return "", fmt.Errorf("not found playback url: %+v", result)
	}

	return result.Args[0].BodyData.VOD.Playback.URL, nil
}

func GetClipMP4URL(client *http.Client, clipID string) (string, error) {
	resp, err := client.Get(fmt.Sprintf("https://ci.me/clips/%s", clipID))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read body: %w", err)
	}

	m := regexp.MustCompile(`(?s)JSON\.parse\((['"].+?['"])\)`).FindSubmatch(data)
	if len(m) < 2 {
		return "", fmt.Errorf("not found data: content-length=%d", len(data))
	}

	v, err := goja.New().RunString(string(m[1]))
	if err != nil {
		return "", fmt.Errorf("failed to parse raw json: %w", err)
	}

	result := &struct {
		Args []struct {
			BodyData struct {
				Clips []struct {
					ID       string `json:"id"`
					Playback struct {
						URL string `json:"url"`
					} `json:"playback"`
				} `json:"clips"`
			} `json:"bodyData"`
		} `json:"args"`
	}{}
	if err = json.Unmarshal([]byte(v.String()), result); err != nil {
		return "", fmt.Errorf("failed to unmarshal data: %w", err)
	}

	for _, args := range result.Args {
		for _, clip := range args.BodyData.Clips {
			if clip.ID == clipID {
				return clip.Playback.URL, nil
			}
		}
	}

	return "", fmt.Errorf("not found playback url: %+v", result)
}

func SignCookie(client *http.Client, payload SignCookieReq) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, "https://ci.me/api/app/streams/sign-cookie", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status code: %s", resp.Status)
	}

	return nil
}
