package kick

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
)

func GetLiveHLSURL(client *http.Client, channelSlug string) (string, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://kick.com/%s", channelSlug), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set(
		"User-Agent",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36",
	)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	liveURLs := regexp.MustCompile(`https://[^"]+?\.m3u8\?token=[A-Za-z0-9._-]+`).FindAllString(string(body), -1)
	if len(liveURLs) <= 0 {
		return "", errors.New("not found m3u8 url")
	}

	return liveURLs[len(liveURLs)-1], nil
}
