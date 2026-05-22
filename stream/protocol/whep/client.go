package whep

import (
	"crypto/sha256"
	"crypto/x509"
	"fmt"
	"log"
	"net/url"
	"reflect"
	"sync"
	"time"

	"github.com/jaesung9507/playgo/secure"
	"github.com/jaesung9507/playgo/stream"
	"github.com/jaesung9507/playgo/stream/codec/h26x"
	"github.com/jaesung9507/playgo/stream/codec/h26x/h264"
	"github.com/jaesung9507/playgo/stream/codec/h26x/h265"

	"github.com/bluenviron/gortsplib/v5/pkg/format/rtph264"
	"github.com/bluenviron/gortsplib/v5/pkg/format/rtph265"
	"github.com/pion/srtp/v3"
	"github.com/pion/webrtc/v4"
)

type Client struct {
	url         *url.URL
	pc          *webrtc.PeerConnection
	packetQueue chan *stream.Packet
	signal      chan any

	ready     bool
	readyCh   chan stream.Codec
	readyOnce sync.Once
}

func New(parsedURL *url.URL) *Client {
	return &Client{
		url:         parsedURL,
		packetQueue: make(chan *stream.Packet, 128),
		signal:      make(chan any, 1),
		readyCh:     make(chan stream.Codec),
	}
}

func (c *Client) readyCodec(codec stream.Codec) {
	c.readyOnce.Do(func() {
		log.Printf("[WHEP] track: %s codec ready", codec.CodecString())
		c.readyCh <- codec
		close(c.readyCh)
		c.ready = true
	})
}

func (c *Client) onPacketH26X(codec h26x.Codec, dtsExtractor h26x.DTSExtractor, au [][]byte, pts int64, clockRate uint32) {
	dts, err := dtsExtractor.Extract(au, pts)
	if err != nil {
		dts = pts
	}

	isKeyFrame, data := codec.ParseAU(au)
	if codec.Ready() {
		c.readyCodec(codec)
	}

	if c.ready && len(data) > 0 {
		ptsTime := time.Duration(pts) * time.Second / time.Duration(clockRate)
		dtsTime := time.Duration(dts) * time.Second / time.Duration(clockRate)
		c.packetQueue <- &stream.Packet{
			IsKeyFrame:      isKeyFrame,
			CompositionTime: ptsTime - dtsTime,
			Time:            dtsTime,
			Data:            data,
		}
	}
}

func (c *Client) Dial() error {
	log.Printf("[WHEP] dial: %s", c.url.String())
	pc, err := webrtc.NewAPI().NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		return err
	}
	c.pc = pc

	if _, err = pc.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo, webrtc.RTPTransceiverInit{
		Direction: webrtc.RTPTransceiverDirectionRecvonly,
	}); err != nil {
		return err
	}

	pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		log.Printf("[WHEP] on track: %s", track.Codec().MimeType)
		switch track.Codec().MimeType {
		case webrtc.MimeTypeH264:
			h264Codec := &h264.Codec{}
			dec := &rtph264.Decoder{}
			dec.Init()
			dtsExtractor := &h264.DTSExtractor{}
			dtsExtractor.Initialize()

			for {
				pkt, _, err := track.ReadRTP()
				if err != nil {
					return
				}

				au, err := dec.Decode(pkt)
				if err != nil || len(au) == 0 {
					continue
				}

				c.onPacketH26X(h264Codec, dtsExtractor, au, int64(pkt.Timestamp), track.Codec().ClockRate)
			}
		case webrtc.MimeTypeH265:
			h265Codec := &h265.Codec{}
			dec := &rtph265.Decoder{}
			dec.Init()
			dtsExtractor := &h265.DTSExtractor{}
			dtsExtractor.Initialize()

			for {
				pkt, _, err := track.ReadRTP()
				if err != nil {
					return
				}

				au, err := dec.Decode(pkt)
				if err != nil || len(au) == 0 {
					continue
				}

				c.onPacketH26X(h265Codec, dtsExtractor, au, int64(pkt.Timestamp), track.Codec().ClockRate)
			}
		default:
			c.signal <- fmt.Errorf("unsupported codec: %s", track.Codec().MimeType)
		}
	})

	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("[WHEP] connection state change: %s", state.String())
		switch state {
		case webrtc.PeerConnectionStateClosed, webrtc.PeerConnectionStateFailed:
			c.signal <- fmt.Errorf("connection closed: %s", state.String())
		}
	})

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		return err
	}

	if err = pc.SetLocalDescription(offer); err != nil {
		return err
	}

	answer, err := Offer((&secure.TLS{}).HTTPClient(), c.url.String(), offer.SDP)
	if err != nil {
		return err
	}

	return pc.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  answer,
	})
}

func (c *Client) Close() {
	log.Print("[WHEP] close")
	if c.pc != nil {
		c.pc.Close()
	}
}

func (c *Client) CodecData() ([]stream.Codec, error) {
	select {
	case codec := <-c.readyCh:
		return []stream.Codec{codec}, nil
	case err := <-c.signal:
		return nil, err.(error)
	}
}

func (c *Client) PacketQueue() <-chan *stream.Packet {
	return c.packetQueue
}

func (c *Client) CloseCh() <-chan any {
	return c.signal
}

func (c *Client) Secure() (bool, bool, map[string]string) {
	if c.pc == nil {
		return false, false, nil
	}

	var transport *webrtc.DTLSTransport
	for _, receiver := range c.pc.GetReceivers() {
		if t := receiver.Transport(); t != nil {
			transport = t
			break
		}
	}

	if transport == nil || transport.State() != webrtc.DTLSTransportStateConnected {
		return false, false, nil
	}

	der := transport.GetRemoteCertificate()
	if len(der) == 0 {
		return false, false, nil
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return false, false, nil
	}

	info := map[string]string{
		"Version":     "DTLS 1.2",
		"Fingerprint": fmt.Sprintf("SHA-256 %X", sha256.Sum256(der)),
		"Subject":     cert.Subject.String(),
		"Issuer":      cert.Issuer.String(),
		"NotBefore":   cert.NotBefore.String(),
		"NotAfter":    cert.NotAfter.String(),
	}

	protectionProfile := reflect.ValueOf(transport).Elem().FieldByName("srtpProtectionProfile")
	if protectionProfile.IsValid() && protectionProfile.CanUint() {
		info["ProtectionProfile"] = srtp.ProtectionProfile(protectionProfile.Uint()).String()
	}

	return true, true, info
}
