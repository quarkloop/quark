package space

import (
	"bytes"
	"io"
	"strings"
	"sync"
	"time"
)

// ringBuf is a fixed-capacity ring buffer of log lines, safe for concurrent use.
//
// Each line is assigned a monotonically increasing sequence number. Consumers
// track the last sequence number they saw and call LinesSince(seq) to get only
// new lines. This is safe even when the buffer wraps: the sequence number never
// resets, so a slow consumer that falls behind the ring simply gets the oldest
// available lines rather than re-reading stale ones.
type ringBuf struct {
	mu    sync.RWMutex
	lines []logLine
	cap   int
	head  int  // index of the oldest entry in lines[]
	size  int  // number of valid entries
	seq   uint64 // monotonic write counter; increments on every appended line
}

type logLine struct {
	t    time.Time
	text string
	seq  uint64
}

func newRingBuf(capacity int) *ringBuf {
	return &ringBuf{
		lines: make([]logLine, capacity),
		cap:   capacity,
	}
}

// Write implements io.Writer. Each write call appends one log line.
func (b *ringBuf) Write(p []byte) (int, error) {
	text := strings.TrimRight(string(p), "\n\r")
	if text == "" {
		return len(p), nil
	}
	b.mu.Lock()
	b.seq++
	idx := (b.head + b.size) % b.cap
	b.lines[idx] = logLine{t: time.Now(), text: text, seq: b.seq}
	if b.size < b.cap {
		b.size++
	} else {
		b.head = (b.head + 1) % b.cap // overwrite oldest
	}
	b.mu.Unlock()
	return len(p), nil
}

// Lines returns all buffered log lines in order, oldest first.
func (b *ringBuf) Lines() []logLine {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]logLine, b.size)
	for i := 0; i < b.size; i++ {
		out[i] = b.lines[(b.head+i)%b.cap]
	}
	return out
}

// LinesSince returns all lines with seq > afterSeq, along with the highest
// sequence number seen. Pass 0 to get all buffered lines.
//
// If the consumer has fallen behind the ring (afterSeq is older than the
// oldest buffered line), all currently-buffered lines are returned so the
// consumer can catch up. The returned nextSeq should be passed as afterSeq
// on the next call.
func (b *ringBuf) LinesSince(afterSeq uint64) (lines []logLine, nextSeq uint64) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	nextSeq = b.seq
	if b.size == 0 {
		return nil, nextSeq
	}

	// Find the oldest entry's seq.
	oldestSeq := b.lines[b.head].seq

	// If the consumer is behind the ring, serve everything from the start.
	startFrom := 0
	if afterSeq >= oldestSeq {
		// Walk from head to find first entry with seq > afterSeq.
		for i := 0; i < b.size; i++ {
			if b.lines[(b.head+i)%b.cap].seq > afterSeq {
				startFrom = i
				break
			}
			startFrom = b.size // all seen
		}
	}

	for i := startFrom; i < b.size; i++ {
		lines = append(lines, b.lines[(b.head+i)%b.cap])
	}
	return lines, nextSeq
}

// Seq returns the current highest sequence number without locking the full read.
func (b *ringBuf) Seq() uint64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.seq
}

// teeWriter writes to both a primary writer and a ring buffer.
type teeWriter struct {
	primary io.Writer
	buf     *ringBuf
	lineBuf bytes.Buffer
	mu      sync.Mutex
}

func newTeeWriter(primary io.Writer, buf *ringBuf) *teeWriter {
	return &teeWriter{primary: primary, buf: buf}
}

// Write forwards bytes to the primary writer and accumulates complete lines
// into the ring buffer.
func (t *teeWriter) Write(p []byte) (int, error) {
	n, err := t.primary.Write(p)

	t.mu.Lock()
	t.lineBuf.Write(p)
	for {
		line, rest, found := bytes.Cut(t.lineBuf.Bytes(), []byte{'\n'})
		if !found {
			break
		}
		t.buf.Write(append(line, '\n'))
		remaining := make([]byte, len(rest))
		copy(remaining, rest)
		t.lineBuf.Reset()
		t.lineBuf.Write(remaining)
	}
	t.mu.Unlock()

	return n, err
}
