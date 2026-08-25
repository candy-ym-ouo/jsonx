package parser

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	jerrors "jsonx/errors"
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
func StreamValues(r io.Reader, opts Options) <-chan *Value {
	values := make(chan *Value)
	go func() {
		stream := NewStreamReader(r, opts)
		for stream.Next() {
			values <- stream.Value()
		}
	}()
	return values
}
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
