package cime

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"

	"github.com/dop251/goja"
)

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
					PlaybackURL string `json:"playbackUrl"`
				} `json:"live"`
			} `json:"bodyData"`
		} `json:"args"`
	}{}
	if err = json.Unmarshal([]byte(v.String()), result); err != nil {
		return "", fmt.Errorf("failed to unmarshal data: %w", err)
	}

	if len(result.Args) < 1 || len(result.Args[0].BodyData.Live.PlaybackURL) <= 0 {
		return "", fmt.Errorf("not found playback url: %+v", result)
	}

	return result.Args[0].BodyData.Live.PlaybackURL, nil
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
					PlaybackURL string `json:"playbackUrl"`
				} `json:"vod"`
			} `json:"bodyData"`
		} `json:"args"`
	}{}
	if err = json.Unmarshal([]byte(v.String()), result); err != nil {
		return "", fmt.Errorf("failed to unmarshal data: %w", err)
	}

	if len(result.Args) < 1 || len(result.Args[0].BodyData.VOD.PlaybackURL) <= 0 {
		return "", fmt.Errorf("not found playback url: %+v", result)
	}

	return result.Args[0].BodyData.VOD.PlaybackURL, nil
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
					ID          string `json:"id"`
					PlaybackURL string `json:"playbackUrl"`
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
				return clip.PlaybackURL, nil
			}
		}
	}

	return "", fmt.Errorf("not found playback url: %+v", result)
}
