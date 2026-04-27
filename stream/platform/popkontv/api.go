package popkontv

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
)

type ClipInfo struct {
	Title   string `json:"vodTitle"`
	Address string `json:"vodAddress"`
}

func GetClipInfo(client *http.Client, videoURL string) (*ClipInfo, error) {
	resp, err := client.Get(videoURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http status code: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	m := regexp.MustCompile(`(?s)<script [^>]*id="__NEXT_DATA__"[^>]*>(.*?)</script>`).FindSubmatch(data)
	if len(m) < 2 {
		return nil, fmt.Errorf("not found data: content-length=%d", len(data))
	}

	result := &struct {
		Props struct {
			PageProps struct {
				ClipDetailFallback struct {
					Data       *ClipInfo `json:"data"`
					StatusCode string    `json:"statusCd"`
					StatusMsg  string    `json:"statusMsg"`
				} `json:"clipDetailFallback"`
			} `json:"pageProps"`
		} `json:"props"`
	}{}
	if err = json.Unmarshal(m[1], result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal data: %w", err)
	}

	clip := result.Props.PageProps.ClipDetailFallback
	if clip.StatusCode != "S2000" || clip.Data == nil {
		return nil, fmt.Errorf("api status code: %s, msg: %s", clip.StatusCode, clip.StatusMsg)
	}

	return clip.Data, nil
}
