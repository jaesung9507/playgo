package file

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/format/flv"
	"github.com/deepch/vdk/format/mp4"
	"github.com/deepch/vdk/format/ts"
)

type LocalFile struct {
	path        string
	closer      io.Closer
	demuxer     av.Demuxer
	signal      chan any
	packetQueue chan *av.Packet
}

func New(filepath string) *LocalFile {
	return &LocalFile{
		path:        filepath,
		signal:      make(chan any),
		packetQueue: make(chan *av.Packet),
	}
}

func (f *LocalFile) getDemuxerFunc() (func(r io.ReadSeeker) (av.Demuxer, error), error) {
	ext := filepath.Ext(path.Base(f.path))
	switch ext {
	case ".flv":
		return func(r io.ReadSeeker) (av.Demuxer, error) { return flv.NewDemuxer(r), nil }, nil
	case ".ts":
		return func(r io.ReadSeeker) (av.Demuxer, error) { return ts.NewDemuxer(r), nil }, nil
	case ".mp4":
		return func(r io.ReadSeeker) (av.Demuxer, error) { return mp4.NewDemuxer(r), nil }, nil
	}
	return nil, fmt.Errorf("unsupported extension: %s", ext)
}

func (f *LocalFile) Dial() error {
	newDemuxer, err := f.getDemuxerFunc()
	if err != nil {
		return err
	}

	file, err := os.Open(f.path)
	if err != nil {
		return err
	}

	if f.demuxer, err = newDemuxer(file); err != nil {
		f.Close()
		return err
	}
	f.closer = file

	return nil
}

func (f *LocalFile) Close() {
	if f.closer != nil {
		f.closer.Close()
	}
}

func (f *LocalFile) CodecData() ([]av.CodecData, error) {
	streams, err := f.demuxer.Streams()
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
	return streams, err
}

func (f *LocalFile) PacketQueue() <-chan *av.Packet {
	return f.packetQueue
}

func (f *LocalFile) CloseCh() <-chan any {
	return f.signal
}
