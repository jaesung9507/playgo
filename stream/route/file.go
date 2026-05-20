package route

import (
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"

	"github.com/deepch/vdk/format/flv"
	"github.com/deepch/vdk/format/mp4"
	"github.com/jaesung9507/playgo/stream"
	"github.com/jaesung9507/playgo/stream/codec/h26x/h264"
	"github.com/jaesung9507/playgo/stream/codec/h26x/h265"
	"github.com/jaesung9507/playgo/stream/format"
	"github.com/jaesung9507/playgo/stream/format/ts"
	"github.com/jaesung9507/playgo/stream/vdk"
)

func getFileDemuxer(ext string) stream.GetFileDemuxerFunc {
	switch ext {
	case ".flv":
		return func(r io.ReadSeeker) stream.Demuxer { return vdk.ToDemuxer(flv.NewDemuxer(r)) }
	case ".ts":
		return func(r io.ReadSeeker) stream.Demuxer { return ts.NewDemuxer(r) }
	case ".mp4":
		return func(r io.ReadSeeker) stream.Demuxer { return vdk.ToDemuxer(mp4.NewDemuxer(r)) }
	case ".h264", ".264":
		return func(r io.ReadSeeker) stream.Demuxer { return h264.NewDemuxer(r) }
	case ".h265", ".265", ".hevc":
		return func(r io.ReadSeeker) stream.Demuxer { return h265.NewDemuxer(r) }
	default:
		return nil
	}
}

func SupportedFilePatterns() string {
	return strings.Join([]string{
		"*.flv",
		"*.ts",
		"*.mp4",
		"*.h264", "*.264",
		"*.h265", "*.265", "*.hevc",
	}, ";")
}

func NewLocalFile(filePath string) (stream.Client, error) {
	ext := filepath.Ext(path.Base(filePath))
	if fn := getFileDemuxer(ext); fn != nil {
		return format.NewLocalFile(filePath, fn), nil
	}

	return nil, fmt.Errorf("unsupported file extension: %s", ext)
}
