package h26x

import (
	"bufio"
	"errors"
	"io"
)

type NALUReader struct {
	r *bufio.Reader
}

func NewNALUReader(r io.Reader) *NALUReader {
	return &NALUReader{r: bufio.NewReader(r)}
}

func (r *NALUReader) Read() ([]byte, error) {
	for {
		p, err := r.r.Peek(4)
		if err != nil {
			return nil, err
		}

		if len(p) >= 3 && p[0] == 0 && p[1] == 0 {
			if p[2] == 1 {
				r.r.Discard(3)
				break
			}
			if len(p) >= 4 && p[2] == 0 && p[3] == 1 {
				r.r.Discard(4)
				break
			}
		}
		r.r.Discard(1)
	}

	var nalu []byte
	for {
		p, err := r.r.Peek(4)
		if len(p) >= 3 && p[0] == 0 && p[1] == 0 && (p[2] == 1 || (len(p) >= 4 && p[2] == 0 && p[3] == 1)) {
			break
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				buf := make([]byte, r.r.Buffered())
				n, _ := r.r.Read(buf)
				nalu = append(nalu, buf[:n]...)
			}
			if len(nalu) == 0 {
				return nil, err
			}
			break
		}

		b, _ := r.r.ReadByte()
		nalu = append(nalu, b)
	}

	return nalu, nil
}
