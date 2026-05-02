package pandatv

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

func GetLiveHLSURL(client *http.Client, userID string) (string, error) {
	req, err := http.NewRequest(http.MethodPost, "https://api.pandalive.co.kr/v1/live/play", bytes.NewReader([]byte((url.Values{
		"userId": []string{userID},
		"action": []string{"watch"},
	}).Encode())))
	if err != nil {
		return "", fmt.Errorf("failed to new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
	req.Header.Set("Origin", "https://www.pandalive.co.kr")
	req.Header.Set("Referer", "https://www.pandalive.co.kr/")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to request: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Result   bool   `json:"result"`
		Msg      string `json:"message"`
		PlayList struct {
			HLS []struct {
				URL string `json:"url"`
			} `json:"hls"`
		} `json:"PlayList"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode json: %w", err)
	}

	if !result.Result || len(result.PlayList.HLS) <= 0 || len(result.PlayList.HLS[0].URL) <= 0 {
		return "", fmt.Errorf("api status result=%t, msg=%q", result.Result, result.Msg)
	}

	return result.PlayList.HLS[0].URL, nil
}
