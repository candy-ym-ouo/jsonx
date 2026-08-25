package parser

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	jerrors "jsonx/errors"
	"sync"
)

// StreamReader incrementally reads root-array elements. Only the current
// element is buffered, so memory use is independent of the array length.
type StreamReader struct {
	reader                      *bufio.Reader
	opts                        Options
	started, array, first, done bool
	current                     *Value
	err                         error
	offset                      int
}

func NewStreamReader(r io.Reader, opts Options) *StreamReader {
	return &StreamReader{reader: bufio.NewReaderSize(r, 32<<10), opts: opts, first: true}
}
// StreamValues incrementally parses the root value or root-array elements of r
// and delivers each *Value on the returned channel.
//
// Lifecycle: the returned channel is closed once parsing completes — whether
// the stream runs to completion, the caller stops early via stop, or the
// underlying reader blocks forever (a pipe, socket, or other reader whose Read
// never returns). In every case the producer goroutine exits rather than
// accumulating per StreamValues invocation.
//
// The returned stop func signals the producer to abandon the remaining input;
// call it when you stop consuming early (for example after the first element).
// stop unblocks the producer whether it is parked on its next channel send or
// blocked inside r.Read, and is idempotent: it is safe to call after the
// channel has already drained, and at most one call has any effect. The caller
// is never required to drain the channel to release the goroutine.
//
// Normal completion closes the channel after the last element, so ranging over
// it until it closes is safe and terminates when the stream ends.
func StreamValues(r io.Reader, opts Options) (<-chan *Value, func()) {
	values := make(chan *Value)
	done := make(chan struct{})
	var once sync.Once
	stop := func() {
		once.Do(func() {
			close(done)
			// If the reader can be closed, close it now so a producer parked
			// inside r.Read (an io.Pipe with no further writes, a socket, ...)
			// returns immediately. select-on-done covers the send path but
			// cannot reach into a synchronous Read; closing the reader unblocks
			// it directly. For non-blocking readers (strings, bytes) Close is a
			// harmless no-op or absent.
			if c, ok := r.(io.Closer); ok {
				_ = c.Close()
			}
		})
	}
	go func() {
		defer close(values)
		stream := NewStreamReader(&cancelableReader{r: r, done: done}, opts)
		for stream.Next() {
			select {
			case values <- stream.Value():
			case <-done:
				return
			}
		}
	}()
	return values, stop
}

// cancelableReader makes a blocking io.Reader interruptible. A single
// long-lived pump goroutine drives r.Read in a loop and hands each result to
// Read through a buffered channel. Read selects between the next result and
// done, so a blocked Read returns immediately when stop is called.
//
// The crucial property is that only one pump goroutine exists per stream, no
// matter how many Read calls StreamReader makes — so there is no per-Read
// goroutine accumulation. When done closes, the pump observes it on its next
// loop iteration and exits; if r.Read is parked when that happens, the pump
// stays parked until r is closed or produces data, but that is at most one
// parked goroutine for the whole stream (collected when r is closed, as it is
// for io.PipeReader and other closeable blocking readers via the stop func's
// Close). For non-blocking readers (strings.NewReader, bytes.Reader) the pump
// never parks, so nothing lingers. cancelableReader does not alter the data or
// buffering StreamReader observes: it forwards bytes and errors verbatim.
type cancelableReader struct {
	r    io.Reader
	done <-chan struct{}

	// Single pump for all Read calls. Allocated lazily on first Read so that a
	// stream whose goroutine never reaches a read (e.g. already done) pays
	// nothing.
	pumpMu  sync.Mutex
	result  chan readResult
	started bool
}

type readResult struct {
	n   int
	err error
	buf []byte
}

// ensurePump starts the single pump goroutine if it has not been started yet.
func (c *cancelableReader) ensurePump() {
	c.pumpMu.Lock()
	defer c.pumpMu.Unlock()
	if c.started {
		return
	}
	c.started = true
	c.result = make(chan readResult, 1)
	go c.pump()
}

// pump calls r.Read in a loop, forwarding each result. It stops when it sees
// done closed between reads (without a pending r.Read) so it can exit cleanly
// after normal end-of-input too.
func (c *cancelableReader) pump() {
	for {
		select {
		case <-c.done:
			return
		default:
		}
		// Issue a Read into a fresh buffer; only the n/err are forwarded, the
		// bytes are copied into the caller's slice by Read below. We cannot let
		// r write directly into the caller's p across the async boundary
		// safely, so Read copies from this local buffer.
		buf := make([]byte, 4096)
		n, err := c.r.Read(buf)
		select {
		case c.result <- readResult{n: n, err: err, buf: buf[:n]}:
		case <-c.done:
			return
		}
	}
}

func (c *cancelableReader) Read(p []byte) (int, error) {
	select {
	case <-c.done:
		return 0, errStreamCanceled
	default:
	}
	c.ensurePump()
	select {
	case res := <-c.result:
		copy(p, res.buf)
		return len(res.buf), res.err
	case <-c.done:
		return 0, errStreamCanceled
	}
}

// errStreamCanceled is the sentinel returned by cancelableReader.Read once stop
// interrupts a blocked read. StreamReader surfaces it through Err() as
// io.ErrUnexpectedEOF via unexpectedEOF, which is the same status a truncated
// stream already reports — so a stopped stream looks like an ended stream to
// the consumer rather than as a spurious parse error.
var errStreamCanceled = errors.New("jsonx: stream values canceled")
func (s *StreamReader) Next() bool {
	if s.err != nil || s.done {
		return false
	}
	if !s.started {
		first, err := s.readNonSpace()
		if err != nil {
			s.finish(err)
			return false
		}
		s.started = true
		if first == '[' {
			s.array = true
		} else {
			return s.parseSingle(first)
		}
	}
	if !s.array {
		s.finish(io.EOF)
		return false
	}
	b, err := s.readNonSpace()
	if err != nil {
		s.finish(unexpectedEOF(err))
		return false
	}
	if s.first {
		s.first = false
		if b == ']' {
			s.finish(s.endArray())
			return false
		}
	} else {
		if b == ']' {
			s.finish(s.endArray())
			return false
		}
		if b != ',' {
			s.finish(fmt.Errorf("jsonx: expected comma between stream elements"))
			return false
		}
		b, err = s.readNonSpace()
		if err != nil {
			s.finish(unexpectedEOF(err))
			return false
		}
		if b == ']' {
			s.finish(fmt.Errorf("jsonx: trailing comma in stream array"))
			return false
		}
	}
	return s.parseElement(b, false)
}
func (s *StreamReader) parseSingle(first byte) bool {
	if !s.parseElement(first, false) {
		return false
	}
	if err := s.expectEOF(); err != nil {
		s.finish(err)
		return false
	}
	s.done = true
	s.err = io.EOF
	return true
}
func (s *StreamReader) endArray() error {
	if err := s.expectEOF(); err != nil {
		return err
	}
	return io.EOF
}
func (s *StreamReader) expectEOF() error {
	if _, err := s.readNonSpace(); err == io.EOF {
		return nil
	} else if err != nil {
		return err
	}
	return jerrors.New(jerrors.ETrailingData, "trailing data after root value")
}
func (s *StreamReader) parseElement(first byte, single bool) bool {
	raw, err := s.readValue(first)
	if err != nil {
		s.finish(err)
		return false
	}
	value, err := Parse(raw, s.opts)
	if err != nil {
		s.finish(err)
		return false
	}
	s.current = value
	if single {
		s.done = true
	}
	return true
}
func (s *StreamReader) readValue(first byte) ([]byte, error) {
	var out bytes.Buffer
	out.WriteByte(first)
	inString, escaped := first == '"', false
	depth := 0
	if first == '{' || first == '[' {
		depth = 1
	}
	for {
		b, err := s.readByte()
		if err != nil {
			if err == io.EOF && depth == 0 && !inString {
				return bytes.TrimSpace(out.Bytes()), nil
			}
			return nil, unexpectedEOF(err)
		}
		if inString {
			out.WriteByte(b)
			if escaped {
				escaped = false
			} else if b == '\\' {
				escaped = true
			} else if b == '"' {
				inString = false
				if depth == 0 {
					return out.Bytes(), nil
				}
			}
			continue
		}
		if b == '"' {
			inString = true
			out.WriteByte(b)
			continue
		}
		if b == '{' || b == '[' {
			depth++
		}
		if b == '}' || b == ']' {
			if depth == 0 {
				if err := s.reader.UnreadByte(); err == nil {
					s.offset--
				}
				return bytes.TrimSpace(out.Bytes()), nil
			}
			depth--
			if depth == 0 {
				out.WriteByte(b)
				return out.Bytes(), nil
			}
		}
		if depth == 0 && (b == ',' || b == ']' || isSpace(b)) {
			if b == ',' || b == ']' {
				if err := s.reader.UnreadByte(); err == nil {
					s.offset--
				}
			}
			return bytes.TrimSpace(out.Bytes()), nil
		}
		out.WriteByte(b)
	}
}
func (s *StreamReader) readNonSpace() (byte, error) {
	for {
		b, err := s.readByte()
		if err != nil {
			return 0, err
		}
		if isSpace(b) {
			continue
		}
		if b == '/' && s.opts.AllowComments {
			next, err := s.readByte()
			if err != nil {
				return 0, err
			}
			switch next {
			case '/':
				for {
					c, err := s.readByte()
					if err != nil {
						return 0, err
					}
					if c == '\n' {
						break
					}
				}
				continue
			case '*':
				previous := byte(0)
				for {
					c, err := s.readByte()
					if err != nil {
						return 0, unexpectedEOF(err)
					}
					if previous == '*' && c == '/' {
						break
					}
					previous = c
				}
				continue
			default:
				if err := s.reader.UnreadByte(); err == nil {
					s.offset--
				}
			}
		}
		return b, nil
	}
}
func (s *StreamReader) readByte() (byte, error) {
	b, err := s.reader.ReadByte()
	if err == nil {
		s.offset++
	}
	return b, err
}
func (s *StreamReader) finish(err error) { s.current = nil; s.done = true; s.err = err }
func (s *StreamReader) Value() *Value    { return s.current }
func (s *StreamReader) Err() error       { return s.err }
func (s *StreamReader) Offset() int      { return s.offset }
func isSpace(b byte) bool                { return b == ' ' || b == '\t' || b == '\r' || b == '\n' }
func unexpectedEOF(err error) error {
	if err == io.EOF {
		return io.ErrUnexpectedEOF
	}
	return err
}
