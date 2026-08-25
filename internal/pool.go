package internal

import (
	"bytes"
	"sync"
)

var bufferPool = sync.Pool{New: func() any { return new(bytes.Buffer) }}

func AcquireBuffer() *bytes.Buffer {
	b := bufferPool.Get().(*bytes.Buffer)
	b.Reset()
	return b
}
func ReleaseBuffer(b *bytes.Buffer) {
	if b == nil {
		return
	}
	// Do not retain unexpectedly huge user payloads in a process-wide pool.
	if b.Cap() <= 256<<10 {
		b.Reset()
		bufferPool.Put(b)
	}
}
func BufferBytes(b *bytes.Buffer) []byte {
	if b == nil {
		return nil
	}
	return b.Bytes()
}

type BytePool struct {
	small  sync.Pool
	medium sync.Pool
	large  sync.Pool
}

func (p *BytePool) Get(size int) []byte {
	var pool *sync.Pool
	capHint := 4 << 10
	if size > capHint {
		pool, capHint = &p.medium, 32<<10
	} else {
		pool = &p.small
	}
	if size > capHint {
		pool, capHint = &p.large, 256<<10
	}
	if v := pool.Get(); v != nil {
		return v.([]byte)[:0]
	}
	return make([]byte, 0, capHint)
}
func (p *BytePool) Put(buf []byte) {
	switch cap(buf) {
	case 4 << 10:
		p.small.Put(buf[:0])
	case 32 << 10:
		p.medium.Put(buf[:0])
	case 256 << 10:
		p.large.Put(buf[:0])
	}
}
