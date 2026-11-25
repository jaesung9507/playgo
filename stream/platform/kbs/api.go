package kbs

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"

	"github.com/dop251/goja"
)

type Channel struct {
	ServiceURL  string `json:"service_url"`
	MediaType   string `json:"media_type"`
	NID         string `json:"nid"`
	Bitrate     string `json:"bitrate"`
	ChannelCode string `json:"channel_code"`
	ChannelID   string `json:"channel_id"`
}

type Contents struct {
	ProgramCode           string `json:"program_code"`
	ProgramID             string `json:"program_id"`
	ContentType           string `json:"content_type"`
	ContentsID            string `json:"contents_id"`
	Title                 string `json:"title"`
	ProgramTitle          string `json:"program_title"`
	ProgramSequenceNumber string `json:"program_sequence_number"`
	ProgramSubtitle       string `json:"program_subtitle"`
	MainStory             string `json:"main_story"`
	MainStoryShort        string `json:"main_story_short"`
	Play1URL              string `json:"play1_url"`
	CDNFileName           string `json:"-"`
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

func GetContents(client *http.Client, contentsURL string) (*Contents, error) {
	resp, err := client.Get(contentsURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	m := regexp.MustCompile(`(?s)StrUtil\.nvl\(('.*?')\)\s*==\s*''\s*\|\|\s*JSON\.parse`).FindSubmatch(data)
	if len(m) < 2 {
		return nil, fmt.Errorf("not found data: content-length=%d", len(data))
	}

	v, err := goja.New().RunString(string(m[1]))
	if err != nil {
		return nil, fmt.Errorf("failed to parse raw json: %w", err)
	}

	result := struct {
		Result string `json:"result"`
		Bill   struct {
			CDNFileName string `json:"cdn_file_name"`
		} `json:"bill_info"`
		Data string `json:"data"`
	}{}
	if err = json.Unmarshal([]byte(v.String()), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal result: %w", err)
	}

	if result.Result != "OK" {
		return nil, fmt.Errorf("api result: %s", result.Result)
	}

	contents := Contents{CDNFileName: result.Bill.CDNFileName}
	if err = json.Unmarshal([]byte(result.Data), &contents); err != nil {
		return nil, fmt.Errorf("failed to unmarshal data: %w", err)
	}

	return &contents, nil
}

func (c *Contents) ContentsTitle() string {
	if c.ProgramSequenceNumber != "" {
		return fmt.Sprintf("%s (%s)", c.ProgramTitle, c.ProgramSequenceNumber)
	}

	return c.Title
}
