package kbs

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type Channel struct {
	ServiceURL  string `json:"service_url"`
	MediaType   string `json:"media_type"`
	NID         string `json:"nid"`
	Bitrate     string `json:"bitrate"`
	ChannelCode string `json:"channel_code"`
	ChannelID   string `json:"channel_id"`
}

func GetChannels(client *http.Client, channelCode string) ([]Channel, error) {
	resp, err := client.Get(fmt.Sprintf("https://cfpwwwapi.kbs.co.kr/api/v1/landing/live/channel_code/%s", channelCode))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	result := struct {
		Ret         int       `json:"ret"`
		Msg         string    `json:"msg"`
		ChannelItem []Channel `json:"channel_item"`
	}{}
	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode json: %w", err)
	}

	if result.Ret > 0 {
		return nil, fmt.Errorf("api status=%d, message=%s", result.Ret, result.Msg)
	}

	return result.ChannelItem, nil
}
