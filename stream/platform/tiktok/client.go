package tiktok

import (
	"errors"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"

	"github.com/jaesung9507/playgo/secure"
	httpStream "github.com/jaesung9507/playgo/stream/protocol/http"

	"github.com/deepch/vdk/av"
)

type Client struct {
	url       *url.URL
	mp4Client *httpStream.MP4Client
	flvClient *httpStream.Client
	tls       *secure.TLS
}

func New(parsedURL *url.URL) *Client {
	return &Client{
		url: parsedURL,
	}
}

type transport struct {
	Transport http.RoundTripper
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Add("Referer", "https://www.tiktok.com/")
	return t.Transport.RoundTrip(req)
}

func (c *Client) Dial() error {
	var tls secure.TLS
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tls.Config(),
		},
	}
	client.Jar, _ = cookiejar.New(nil)

	log.Printf("[TikTok] dial: %s", c.url.String())
	var mp4URL, flvURL *url.URL
	if m := regexp.MustCompile(`^/@([^/?]+)/video/(\d+)/?$`).FindStringSubmatch(c.url.Path); m != nil {
		rawURL, err := GetVideoMP4URL(client, c.url.String())
		if err != nil {
			return err
		}

		mp4URL, err = url.Parse(rawURL)
		if err != nil {
			return err
		}
	} else if m := regexp.MustCompile(`^/@([^/?]+)(/live)?/?$`).FindStringSubmatch(c.url.Path); m != nil {
		rawURL, err := GetLiveFLVURL(client, m[1])
		if err != nil {
			return err
		}

		flvURL, err = url.Parse(rawURL)
		if err != nil {
			return err
		}
	}

	if mp4URL != nil {
		c.mp4Client = httpStream.NewMP4Client(mp4URL)
		client.Transport = &transport{Transport: client.Transport}
		c.tls = &tls
		return c.mp4Client.DialWithHTTPClient(client)
	} else if flvURL != nil {
		c.flvClient = httpStream.New(flvURL)
		return c.flvClient.Dial()
	}

	return errors.New("not supported url")
}

func (c *Client) Close() {
	log.Print("[TikTok] close")
	if c.mp4Client != nil {
		c.mp4Client.Close()
	}

	if c.flvClient != nil {
		c.flvClient.Close()
	}
}

func (c *Client) CodecData() ([]av.CodecData, error) {
	if c.mp4Client != nil {
		return c.mp4Client.CodecData()
	}

	if c.flvClient != nil {
		return c.flvClient.CodecData()
	}

	return nil, errors.New("not supported")
}

func (c *Client) PacketQueue() <-chan *av.Packet {
	if c.mp4Client != nil {
		return c.mp4Client.PacketQueue()
	}

	if c.flvClient != nil {
		return c.flvClient.PacketQueue()
	}

	return nil
}

func (c *Client) CloseCh() <-chan any {
	if c.mp4Client != nil {
		return c.mp4Client.CloseCh()
	}

	if c.flvClient != nil {
		return c.flvClient.CloseCh()
	}

	return nil
}

func (c *Client) Secure() (bool, bool, map[string]string) {
	if c.tls != nil {
		return c.tls.Info()
	}

	if c.mp4Client != nil {
		return c.mp4Client.Secure()
	}

	if c.flvClient != nil {
		return c.flvClient.Secure()
	}

	return false, false, nil
}
