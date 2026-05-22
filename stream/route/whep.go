package route

import (
	"net/http"
	"strings"

	"github.com/jaesung9507/playgo/secure"
)

func CheckWHEP(rawURL string) bool {
	req, err := http.NewRequest(http.MethodOptions, rawURL, nil)
	if err != nil {
		return false
	}

	resp, err := (&secure.TLS{}).HTTPClient().Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	acceptPost := strings.ToLower(resp.Header.Get("Accept-Post"))
	return strings.Contains(acceptPost, "application/sdp")
}
