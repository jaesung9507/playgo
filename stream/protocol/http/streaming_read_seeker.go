package http

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
)

const (
	chunkSize         = 1 * 1024 * 1024
	fetchChunk        = 8 * 1024 * 1024
	tailSize          = 2 * 1024 * 1024
	prefetchThreshold = 4 * 1024 * 1024
)

type rangeBlock struct {
	start int64
	end   int64
}

type StreamingReadSeeker struct {
	url            string
	size           int64
	offset         int64
	mu             sync.Mutex
	cond           *sync.Cond
	chunks         map[int64][]byte
	inFlightRanges []rangeBlock
	downloaded     []rangeBlock
	client         *http.Client
	ctx            context.Context
	cancel         context.CancelFunc
}

func NewStreamingReadSeeker(url string, client *http.Client) (*StreamingReadSeeker, error) {
	ctx, cancel := context.WithCancel(context.Background())

	s := &StreamingReadSeeker{
		url:    url,
		client: client,
		chunks: make(map[int64][]byte),
		ctx:    ctx,
		cancel: cancel,
	}
	s.cond = sync.NewCond(&s.mu)

	if err := s.checkRangeSupport(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to check range support: %w", err)
	}

	if s.size <= 0 {
		resp, err := s.client.Head(s.url)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("failed to get file size: %w", err)
		}
		defer resp.Body.Close()
		s.size = resp.ContentLength
		log.Printf("[STREAMING-MP4] get file size From HEAD: %d bytes\n", s.size)
	}

	if err := s.initialDownload(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initial download: %w", err)
	}

	return s, nil
}

func (s *StreamingReadSeeker) checkRangeSupport() error {
	req, err := http.NewRequest(http.MethodGet, s.url, nil)
	if err != nil {
		return fmt.Errorf("failed to new request: %w", err)
	}
	req.Header.Set("Range", "bytes=0-0")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("status code: %d", resp.StatusCode)
	}

	if _, after, found := strings.Cut(resp.Header.Get("Content-Range"), "/"); found {
		s.size, _ = strconv.ParseInt(after, 10, 64)
		log.Printf("[STREAMING-MP4] get file size From Range GET: %d bytes\n", s.size)
	}

	return nil
}

func (s *StreamingReadSeeker) initialDownload() error {
	tailStart := max(s.size-tailSize, 0)

	ch := make(chan error, 2)
	go func() {
		ch <- s.DownloadRange(0, fetchChunk)
	}()
	go func() {
		ch <- s.DownloadRange(tailStart, s.size)
	}()

	if err := <-ch; err == nil {
		return nil
	}

	return <-ch
}

func (s *StreamingReadSeeker) writeAt(offset int64, data []byte) {
	remain := data
	currOffset := offset
	for len(remain) > 0 {
		chunkIndex := currOffset / chunkSize
		chunkOffset := currOffset % chunkSize

		chunk, ok := s.chunks[chunkIndex]
		if !ok {
			chunk = make([]byte, chunkSize)
			s.chunks[chunkIndex] = chunk
		}

		n := min(int(chunkSize-chunkOffset), len(remain))
		copy(chunk[chunkOffset:], remain[:n])

		currOffset += int64(n)
		remain = remain[n:]
	}
}

func (s *StreamingReadSeeker) readAt(p []byte, offset int64) int {
	totalRead := 0
	toRead := len(p)
	currOffset := offset

	for totalRead < toRead {
		if currOffset >= s.size {
			break
		}
		chunkIndex := currOffset / chunkSize
		chunkOffset := currOffset % chunkSize

		chunk, ok := s.chunks[chunkIndex]
		if !ok {
			break
		}

		avail := min(int(chunkSize-chunkOffset), (toRead - totalRead))
		if currOffset+int64(avail) > s.size {
			avail = int(s.size - currOffset)
		}

		if avail <= 0 {
			break
		}
		copy(p[totalRead:], chunk[chunkOffset:chunkOffset+int64(avail)])
		totalRead += avail
		currOffset += int64(avail)
	}

	return totalRead
}

func (s *StreamingReadSeeker) isInFlight(pos int64) bool {
	for _, r := range s.inFlightRanges {
		if pos >= r.start && pos < r.end {
			return true
		}
	}

	return false
}

func (s *StreamingReadSeeker) DownloadRange(start, end int64) error {
	start = max(start, 0)
	end = min(end, s.size)
	if start >= end {
		return nil
	}

	alignedStart := (start / chunkSize) * chunkSize
	alignedEnd := min(alignedStart+fetchChunk, s.size)
	if end > alignedEnd {
		alignedEnd = min(((end+chunkSize-1)/chunkSize)*chunkSize, s.size)
	}

	s.mu.Lock()
	if s.isInFlight(alignedStart) {
		s.mu.Unlock()
		return nil
	}

	s.inFlightRanges = append(s.inFlightRanges, rangeBlock{start: alignedStart, end: alignedEnd})
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		for i, r := range s.inFlightRanges {
			if r.start == alignedStart && r.end == alignedEnd {
				s.inFlightRanges = append(s.inFlightRanges[:i], s.inFlightRanges[i+1:]...)
				break
			}
		}
		s.cond.Broadcast()
		s.mu.Unlock()
	}()

	log.Printf("[STREAMING-MP4] http range request: bytes=%d-%d\n", alignedStart, alignedEnd)

	req, err := http.NewRequestWithContext(s.ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return fmt.Errorf("failed to new request: %w", err)
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", alignedStart, alignedEnd-1))

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("status code: %d", resp.StatusCode)
	}

	curr := alignedStart
	buf := make([]byte, 128*1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			s.mu.Lock()
			s.writeAt(curr, buf[:n])
			s.addRange(curr, curr+int64(n))
			curr += int64(n)
			s.cond.Broadcast()
			s.mu.Unlock()
		}
		if err != nil {
			break
		}
	}

	return nil
}

func (s *StreamingReadSeeker) addRange(start, end int64) {
	s.downloaded = append(s.downloaded, rangeBlock{start, end})
	sort.Slice(s.downloaded, func(i, j int) bool {
		return s.downloaded[i].start < s.downloaded[j].start
	})

	var merged []rangeBlock
	curr := s.downloaded[0]
	for i := 1; i < len(s.downloaded); i++ {
		if s.downloaded[i].start <= curr.end {
			if s.downloaded[i].end > curr.end {
				curr.end = s.downloaded[i].end
			}
		} else {
			merged = append(merged, curr)
			curr = s.downloaded[i]
		}
	}
	merged = append(merged, curr)
	s.downloaded = merged
}

func (s *StreamingReadSeeker) isDownloaded(start, end int64) bool {
	if start >= end {
		return true
	}

	checkEnd := min(end, s.size)
	for _, r := range s.downloaded {
		if start >= r.start && start < r.end {
			if checkEnd <= r.end {
				return true
			}
		}
	}

	return false
}

func (s *StreamingReadSeeker) Read(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.offset >= s.size {
		return 0, io.EOF
	}

	for s.ctx.Err() == nil {
		start := s.offset
		if s.isDownloaded(start, start+1) {
			var availEnd int64
			for _, r := range s.downloaded {
				if start >= r.start && start < r.end {
					availEnd = r.end
					break
				}
			}

			readSize := int64(len(p))
			if start+readSize > availEnd {
				readSize = availEnd - start
			}
			if start+readSize > s.size {
				readSize = s.size - start
			}

			if readSize > 0 {
				n := s.readAt(p[:readSize], start)
				if n > 0 {
					s.offset += int64(n)
					s.triggerPrefetch()
					s.purgeConsumedChunks()
					return n, nil
				}
			}
		}

		if !s.isInFlight(start) {
			go s.DownloadRange(start, start+fetchChunk)
		}
		s.cond.Wait()
	}

	return 0, io.EOF
}

func (s *StreamingReadSeeker) purgeConsumedChunks() {
	currentChunkIdx := s.offset / chunkSize
	limit := (currentChunkIdx - 1) * chunkSize

	removed := false
	for idx := range s.chunks {
		if idx*chunkSize < fetchChunk || idx*chunkSize > s.size-tailSize {
			continue
		}

		if idx*chunkSize < limit {
			log.Printf("[STREAMING-MP4] delete chunk index: %d", idx)
			delete(s.chunks, idx)
			removed = true
		}
	}

	if removed {
		s.trimDownloadedRanges(limit)
	}
}

func (s *StreamingReadSeeker) trimDownloadedRanges(beforeOffset int64) {
	var nextDownloaded []rangeBlock
	for _, r := range s.downloaded {
		if r.end <= fetchChunk {
			nextDownloaded = append(nextDownloaded, r)
			continue
		}

		if r.end <= beforeOffset {
			if r.start < fetchChunk {
				nextDownloaded = append(nextDownloaded, rangeBlock{start: r.start, end: fetchChunk})
			}
			continue
		}

		if r.start < beforeOffset {
			if r.start < fetchChunk {
				nextDownloaded = append(nextDownloaded, rangeBlock{start: r.start, end: fetchChunk})
			}
			nextDownloaded = append(nextDownloaded, rangeBlock{start: beforeOffset, end: r.end})
			continue
		}

		nextDownloaded = append(nextDownloaded, r)
	}
	s.downloaded = nextDownloaded
}

func (s *StreamingReadSeeker) triggerPrefetch() {
	nextCheck := min(s.offset+prefetchThreshold, s.size)
	if !s.isDownloaded(s.offset, nextCheck) && !s.isInFlight(s.offset) {
		fetchStart := s.offset
		for _, r := range s.downloaded {
			if s.offset >= r.start && s.offset < r.end {
				fetchStart = r.end
				break
			}
		}
		if fetchStart < s.size && !s.isInFlight(fetchStart) {
			go s.DownloadRange(fetchStart, fetchStart+fetchChunk)
		}
	}
}

func (s *StreamingReadSeeker) Seek(offset int64, whence int) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var newOffset int64
	switch whence {
	case io.SeekStart:
		newOffset = offset
	case io.SeekCurrent:
		newOffset = s.offset + offset
	case io.SeekEnd:
		newOffset = s.size + offset
	default:
		return 0, fmt.Errorf("invalid whence: %d", whence)
	}

	if newOffset < 0 || newOffset > s.size {
		return 0, fmt.Errorf("seek out of range: %d", newOffset)
	}
	if newOffset != s.offset {
		s.offset = newOffset
		s.cond.Broadcast()
	}

	return s.offset, nil
}

func (s *StreamingReadSeeker) Close() error {
	log.Print("[STREAMING-MP4] close")
	s.cancel()
	s.mu.Lock()
	s.cond.Broadcast()
	s.mu.Unlock()
	return nil
}
