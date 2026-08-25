package jsonx_test

import (
	"io"
	"jsonx/jsonx"
	"jsonx/parser"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestProbeNoDrainStability exercises the common early-stop path: a fast,
// non-blocking reader (strings.NewReader), consume one element, call stop, and
// deliberately do NOT drain. The producer (and any reader-pump helper) must
// exit on stop alone, so the live goroutine count must not climb across rounds.
func TestProbeNoDrainStability(t *testing.T) {
	runtime.GC()
	baseline := runtime.NumGoroutine()
	for i := 0; i < 200; i++ {
		ch, stop := jsonx.StreamValues(strings.NewReader(`[1,2,3,4,5]`))
		<-ch
		stop()
		// deliberately do NOT drain ch
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() <= baseline+1 {
			return
		}
		runtime.Gosched()
	}
	t.Fatalf("no-drain leak: baseline=%d now=%d", baseline, runtime.NumGoroutine())
}

// TestProbeBlockingReaderEarlyStop uses an io.Pipe (a closeable, blocking
// reader). The consumer stops after the first element while the producer is
// blocked inside r.Read. stop() must close the reader, unblock the producer,
// and close the channel — with no draining from the caller.
func TestProbeBlockingReaderEarlyStop(t *testing.T) {
	pr, pw := io.Pipe()
	ch, stop := jsonx.StreamValues(pr)
	// Write only the first element + comma, then hold the pipe open (no more writes).
	if _, err := pw.Write([]byte("[1,")); err != nil {
		t.Fatal(err)
	}
	v, ok := <-ch
	if !ok {
		t.Fatal("expected first value")
	}
	if n, _ := v.Int64(); n != 1 {
		t.Fatalf("first = %d", n)
	}
	stop()

	done := make(chan struct{})
	go func() {
		for range ch {
		}
		close(done)
	}()
	select {
	case <-done:
		// producer exited despite being blocked in r.Read() -> good
	case <-time.After(2 * time.Second):
		pw.Close()
		<-done
		t.Fatal("producer did not exit after stop (blocked in r.Read)")
	}
	pw.Close()
}

// TestProbeBlockingReaderNoLeak repeats the closeable-blocking early-stop across
// many rounds and asserts the goroutine count returns to baseline each time —
// i.e. the stop func's Close lets the parked producer/pump exit, so nothing
// accumulates. This is the regression test for the original bug, which leaked
// one goroutine per StreamValues call against a blocking reader.
func TestProbeBlockingReaderNoLeak(t *testing.T) {
	runtime.GC()
	baseline := runtime.NumGoroutine()
	for i := 0; i < 50; i++ {
		pr, pw := io.Pipe()
		ch, stop := jsonx.StreamValues(pr)
		if _, err := pw.Write([]byte("[1,")); err != nil {
			t.Fatal(err)
		}
		<-ch
		stop()
		pw.Close()
		// Wait for this round's channel to close before starting the next, so
		// the count we read reflects a fully settled round.
		deadline := time.Now().Add(1 * time.Second)
		for time.Now().Before(deadline) {
			if _, ok := tryRecv(ch); ok {
				break
			}
			runtime.Gosched()
		}
	}
	// Allow the final goroutine to schedule its exit, then assert no growth.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() <= baseline+1 {
			return
		}
		runtime.Gosched()
	}
	if runtime.NumGoroutine() > baseline+1 {
		t.Fatalf("blocking-reader leak: baseline=%d now=%d", baseline, runtime.NumGoroutine())
	}
}

func tryRecv(ch <-chan *parser.Value) (struct{}, bool) {
	select {
	case _, ok := <-ch:
		_ = ok
		return struct{}{}, true
	default:
		return struct{}{}, false
	}
}

var _ sync.WaitGroup
