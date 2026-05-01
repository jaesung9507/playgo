package popkontv

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
)

type LiveInfo struct {
	CastType      string `json:"castType"`
	CastStartDate string `json:"mc_castStartDate"`
	Title         string `json:"mc_pTitle"`
	SignID        string `json:"mc_signId"`
	PartnerCode   string `json:"mc_partnerCode"`
}

type ClipInfo struct {
	Title   string `json:"vodTitle"`
	Address string `json:"vodAddress"`
}

func (i *LiveInfo) GetHLSURL(client *http.Client) (string, error) {
	data, err := json.Marshal(map[string]any{
		"androidStore":    0,
		"castCode":        fmt.Sprintf("%s-%s", i.SignID, i.CastStartDate),
		"castPartnerCode": i.PartnerCode,
		"castSignId":      i.SignID,
		"castType":        i.CastType,
		"commandType":     0,
		"exePath":         5,
		"partnerCode":     i.PartnerCode,
		"password":        "",
		"version":         "4.6.2",
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal json: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, "https://www.popkontv.com/api/proxy/broadcast/v1/castwatchonoffguest", bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("failed to new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://www.popkontv.com")
	req.Header.Set("Referer", fmt.Sprintf("https://www.popkontv.com/live/view?castId=%s&partnerCode=%s", i.SignID, i.PartnerCode))
	req.Header.Set("clientkey", "Client FpAhe6mh8Qtz116OENBmRddbYVirNKasktdXQiuHfm88zRaFydTsFy63tzkdZY0u")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to request: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		StatusCode string `json:"statusCd"`
		StatusMsg  string `json:"statusMsg"`
		Data       struct {
			CastHLSURL string `json:"castHlsUrl"`
		} `json:"data"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode json: %w", err)
	}

	if len(result.Data.CastHLSURL) <= 0 {
		return "", fmt.Errorf("not found hls url: status code=%q msg=%q", result.StatusCode, result.StatusMsg)
	}

	return result.Data.CastHLSURL, nil
}

func GetLiveInfo(client *http.Client, videoURL string) (*LiveInfo, error) {
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
				McData struct {
					Data       *LiveInfo `json:"data"`
					StatusCode string    `json:"statusCd"`
					StatusMsg  string    `json:"statusMsg"`
				} `json:"mcData"`
			} `json:"pageProps"`
		} `json:"props"`
	}{}
	if err = json.Unmarshal(m[1], result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal data: %w", err)
	}

	mcdata := result.Props.PageProps.McData
	if mcdata.StatusCode != "S2000" || mcdata.Data == nil {
		return nil, fmt.Errorf("api status code=%q, msg=%q", mcdata.StatusCode, mcdata.StatusMsg)
	}

	return mcdata.Data, nil
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
		return nil, fmt.Errorf("api status code=%q, msg=%q", clip.StatusCode, clip.StatusMsg)
	}

	return clip.Data, nil
}
