package format

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/jaesung9507/playgo/stream"
)

type LocalFile struct {
	path        string
	closer      io.Closer
	demuxer     stream.Demuxer
	signal      chan any
	packetQueue chan *stream.Packet
	getDemuxer  stream.GetFileDemuxerFunc
}

func NewLocalFile(filePath string, getDemuxer stream.GetFileDemuxerFunc) *LocalFile {
	switch runtime.GOOS {
	case "windows":
		if len(filePath) > 0 && filePath[0] == '/' {
			filePath = filepath.FromSlash(filePath[1:])
		}
	}

	return &LocalFile{
		path:        filePath,
		signal:      make(chan any, 1),
		packetQueue: make(chan *stream.Packet),
		getDemuxer:  getDemuxer,
	}
}

func (f *LocalFile) Dial() error {
	file, err := os.Open(f.path)
	if err != nil {
		return err
	}
	f.demuxer = f.getDemuxer(file)
	f.closer = file

	return nil
}

func (f *LocalFile) Close() {
	if f.closer != nil {
		f.closer.Close()
	}
}

func (f *LocalFile) CodecData() ([]stream.Codec, error) {
	codecs, err := f.demuxer.CodecData()
	if err == nil {
		go func() {
			for {
				packet, err := f.demuxer.ReadPacket()
				if err != nil {
					if !errors.Is(err, io.EOF) {
						f.signal <- err
					}
					return
				}
				f.packetQueue <- &packet
			}
		}()
	}

	return codecs, err
}

func (f *LocalFile) PacketQueue() <-chan *stream.Packet {
	return f.packetQueue
}

func (f *LocalFile) CloseCh() <-chan any {
	return f.signal
}

func (f *LocalFile) Secure() (bool, bool, map[string]string) {
	return false, false, nil
}
