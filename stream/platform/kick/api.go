package kick

import (
	"encoding/json"
	"fmt"
	"net/http"
)

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36"

type Channel struct {
	ID             int64  `json:"id"`
	UserID         int64  `json:"user_id"`
	FollowersCount string `json:"followers_count"`
}

type LiveStream struct {
	Title       string `json:"session_title"`
	PlaybackURL string `json:"playback_url"`
}

type Video struct {
	Title        string `json:"title"`
	RecordingURL string `json:"recording_url"`
}

type Clip struct {
	Title   string `json:"title"`
	ClipURL string `json:"clip_url"`
}

func GetChannel(client *http.Client, channelSlug string) (*Channel, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://kick.com/api/v2/channels/%s", channelSlug), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result Channel
	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
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

	result := struct {
		Data LiveStream `json:"data"`
	}{}
	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result.Data, nil
}

func GetVideo(client *http.Client, channelID int64, uuid string) (*Video, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://web.kick.com/api/v1/channels/%d/videos/%s", channelID, uuid), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to new request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to request: %w", err)
	}
	defer resp.Body.Close()

	result := struct {
		Data json.RawMessage `json:"data"`
		Msg  string          `json:"message"`
	}{}
	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode body: %w", err)
	}

	if result.Msg != "Success" {
		data := struct {
			Type string `json:"type"`
		}{}
		if err = json.Unmarshal(result.Data, &data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal error: %w", err)
		}
		if len(data.Type) <= 0 {
			data.Type = "UNKNOWN"
		}

		return nil, fmt.Errorf("api error type: %s", data.Type)
	}

	var video Video
	if err = json.Unmarshal(result.Data, &video); err != nil {
		return nil, fmt.Errorf("failed to unmarshal video: %w", err)
	}

	return &video, nil
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

	result := struct {
		Clip Clip `json:"clip"`
	}{}
	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result.Clip, nil
}
