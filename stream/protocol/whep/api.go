package whep

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

func Offer(client *http.Client, url, sdp string) (string, error) {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(sdp))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/sdp")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status: %s", resp.Status)
	}

	answer, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(answer), nil
}
