package sbs

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
)

type Response struct {
	Info struct {
		Title       string `json:"title"`
		ChannelName string `json:"channelname"`
		ChannelID   string `json:"channelid"`
		OnAirYN     string `json:"onair_yn"`
	} `json:"info"`
	Source struct {
		MediaSource struct {
			MediaURL string `json:"mediaurl"`
		} `json:"mediasource"`
	} `json:"source"`
}

func (o *Response) HLSURL() string {
	return o.Source.MediaSource.MediaURL
}

func getChannelPath(channelID string) string {
	if m := regexp.MustCompile(`\d+$`).FindString(channelID); len(m) > 0 {
		if num, err := strconv.Atoi(m); err == nil {
			if num >= 20 {
				return "/binge-watch"
			}
		}
	}

	return ""
}

func GetOnAir(client *http.Client, channelID string) (*Response, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://apis.sbs.co.kr/play-api/1.0/onair%s/channel/%s", getChannelPath(channelID), channelID), nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Set("v_type", "2")
	q.Set("platform", "pcweb")
	q.Set("protocol", "hls")
	q.Set("jwt-token", "")
	q.Set("ssl", "Y")
	q.Set("rscuse", "")
	req.URL.RawQuery = q.Encode()

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	result := &struct {
		Status  int      `json:"status"`
		Message string   `json:"message"`
		OnAir   Response `json:"onair"`
	}{}
	if err = json.NewDecoder(resp.Body).Decode(result); err != nil {
		return nil, fmt.Errorf("failed to decode json: %w", err)
	}

	if result.Status > 0 {
		return nil, fmt.Errorf("api status=%d, message=%s", result.Status, result.Message)
	}

	return &result.OnAir, nil
}

func GetVOD(client *http.Client, mediaID string) (*Response, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://apis.sbs.co.kr/play-api/1.0/sbs_vodall/%s", mediaID), nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Set("jwt-token", "")
	q.Set("platform", "pcweb")
	q.Set("service", "program")
	q.Set("absolute_show", "Y")
	q.Set("ssl", "Y")
	q.Set("rscuse", "")
	q.Set("protocol", "hls")
	req.URL.RawQuery = q.Encode()

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	result := &struct {
		Status  int      `json:"status"`
		Message string   `json:"message"`
		VOD     Response `json:"vod"`
	}{}
	if err = json.NewDecoder(resp.Body).Decode(result); err != nil {
		return nil, fmt.Errorf("failed to decode json: %w", err)
	}

	if result.Status > 0 {
		return nil, fmt.Errorf("api status=%d, message=%s", result.Status, result.Message)
	}

	return &result.VOD, nil
}
