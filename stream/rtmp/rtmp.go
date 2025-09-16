package rtmp

import (
	"crypto/tls"
	"net"
	"net/url"
	"time"

	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/format/rtmp"
)

const (
	DefaultRtmpPort  = ":1935"
	DefaultRtmpsPort = ":443"
)

type RTMPClient struct {
	url         *url.URL
	conn        *rtmp.Conn
	signal      chan any
	packetQueue chan *av.Packet
}

func New(parsedUrl *url.URL) *RTMPClient {
	return &RTMPClient{
		url:         parsedUrl,
		signal:      make(chan any, 1),
		packetQueue: make(chan *av.Packet),
	}
}

func (r *RTMPClient) Dial() error {
	if _, _, err := net.SplitHostPort(r.url.Host); err != nil {
		if r.url.Scheme == "rtmps" {
			r.url.Host += DefaultRtmpsPort
		} else {
			r.url.Host += DefaultRtmpPort
		}
	}

	conn, err := net.Dial("tcp", r.url.Host)
	if err != nil {
		return err
	}

	if r.url.Scheme == "rtmps" {
		tlsConn := tls.Client(conn, &tls.Config{
			InsecureSkipVerify: true,
		})

		if err = tlsConn.Handshake(); err != nil {
			return err
		}
		conn = tlsConn
	}

	r.conn = rtmp.NewConn(conn)
	r.conn.URL = r.url
	return nil
}

func (r *RTMPClient) Close() {
	if r.conn != nil {
		r.conn.Close()
	}
}

func (r *RTMPClient) CodecData() ([]av.CodecData, error) {
	if err := r.conn.NetConn().SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return nil, err
	}
	streams, err := r.conn.Streams()
	if err == nil {
		go func() {
			for {
				if err := r.conn.NetConn().SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
					r.signal <- err
					return
				}
				packet, err := r.conn.ReadPacket()
				if err != nil {
					r.signal <- err
					return
				}
				r.packetQueue <- &packet
			}
		}()
	}
	return streams, err
}

func (r *RTMPClient) PacketQueue() <-chan *av.Packet {
	return r.packetQueue
}

func (r *RTMPClient) CloseCh() <-chan any {
	return r.signal
}
