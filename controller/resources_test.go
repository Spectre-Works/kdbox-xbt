package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestBoundedLogRotatesWithoutLosingSupervisorsDescriptor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "controller.log")
	os.WriteFile(path, bytes.Repeat([]byte("old"), 10000), 0600)
	descriptor, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer descriptor.Close()
	l := &boundedLog{path: path, maximum: 1024}
	for i := 0; i < 1000; i++ {
		if _, err = l.Write([]byte(fmt.Sprintf("entry %d\n", i))); err != nil {
			t.Fatal(err)
		}
	}
	descriptor.Write([]byte("stderr still visible\n"))
	data, _ := os.ReadFile(path)
	if !bytes.Contains(data, []byte("stderr still visible")) {
		t.Fatal("rotation orphaned stderr")
	}
	for _, p := range []string{path, path + ".1"} {
		info, err := os.Stat(p)
		if err != nil || info.Size() > 1024 {
			t.Fatal("unbounded log", p, err)
		}
	}
	if n, err := l.Write(bytes.Repeat([]byte("x"), 4096)); err != nil || n != 4096 {
		t.Fatal(n, err)
	}
	info, _ := os.Stat(path)
	if info.Size() > 1024 {
		t.Fatal("oversized single entry")
	}
}
func TestHeartbeatReadersNeverObserveTruncatedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "heartbeat")
	now := time.Now()
	if err := writeHeartbeat(path, now); err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	group.Add(1)
	go func() {
		defer group.Done()
		for i := 0; i < 2000; i++ {
			if err := writeHeartbeat(path, now); err != nil {
				t.Error(err)
				return
			}
		}
	}()
	for i := 0; i < 2000; i++ {
		data, err := os.ReadFile(path)
		if err != nil || len(data) < 10 {
			t.Fatal("partial heartbeat", string(data), err)
		}
	}
	group.Wait()
}
func TestHTTPConnectionBudgetClosesWithoutLeakingAcceptOrSlots(t *testing.T) {
	base, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	l := newBoundedListener(base, 1)
	defer l.Close()
	first, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	accepted, err := l.Accept()
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := l.Accept(); done <- err }()
	select {
	case <-done:
		t.Fatal("connection cap bypassed")
	case <-time.After(10 * time.Millisecond):
	}
	l.Close()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("closed listener returned a connection")
		}
	case <-time.After(time.Second):
		t.Fatal("Accept leaked after listener close")
	}
	accepted.Close()
	accepted.Close()
	if len(l.slots) != 0 {
		t.Fatal("connection slot leaked")
	}
}
func TestDiagnosticsBudgetAndCancellationReleaseWithoutHoldingSafetyLock(t *testing.T) {
	began := make(chan struct{}, 2)
	stock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user/login" {
			io.WriteString(w, `{"JWT Token":"owner"}`)
			return
		}
		began <- struct{}{}
		<-r.Context().Done()
	}))
	defer stock.Close()
	c := &Controller{backend: stock.URL, client: stock.Client(), sessions: map[string]Session{"owner": {time.Now().Add(time.Hour)}}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request := httptest.NewRequest("GET", "/api/diagnostics?kind=meminfo", nil).WithContext(ctx)
	request.AddCookie(&http.Cookie{Name: "kdbox-xbt", Value: "owner"})
	done := make(chan struct{})
	go func() { c.handler(httptest.NewRecorder(), request); close(done) }()
	<-began
	if !c.mu.TryLock() {
		t.Fatal("diagnostics held safety lock")
	}
	c.mu.Unlock()
	second := httptest.NewRequest("GET", "/api/diagnostics?kind=meminfo", nil)
	second.AddCookie(&http.Cookie{Name: "kdbox-xbt", Value: "owner"})
	out := httptest.NewRecorder()
	c.handler(out, second)
	if out.Code != 503 || out.Header().Get("Retry-After") == "" {
		t.Fatal("parallel diagnostic not bounded", out.Code)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("canceled report did not finish")
	}
	if !c.diagnosticMu.TryLock() {
		t.Fatal("diagnostic slot leaked")
	}
	c.diagnosticMu.Unlock()
}
func TestSlowDashboardResponsesDoNotHoldSafetyLock(t *testing.T) {
	for _, path := range []string{"/api/status", "/api/pool-status", "/api/system"} {
		t.Run(path, func(t *testing.T) {
			stock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/mcb/pools" {
					io.WriteString(w, `[]`)
				} else {
					io.WriteString(w, `{}`)
				}
			}))
			defer stock.Close()
			c := &Controller{backend: stock.URL, client: stock.Client(), sessions: map[string]Session{"owner": {time.Now().Add(time.Hour)}}}
			// The response writer blocks after the snapshot/native query has completed.
			writer := &blockingResponse{header: make(http.Header), entered: make(chan struct{}), resume: make(chan struct{})}
			request := httptest.NewRequest("GET", path, nil)
			request.AddCookie(&http.Cookie{Name: "kdbox-xbt", Value: "owner"})
			done := make(chan struct{})
			go func() { c.handler(writer, request); close(done) }()
			select {
			case <-writer.entered:
			case <-time.After(time.Second):
				t.Fatal("response did not start")
			}
			ok := c.mu.TryLock()
			if ok {
				c.mu.Unlock()
			}
			close(writer.resume)
			<-done
			if !ok {
				t.Fatal("slow browser held safety lock")
			}
		})
	}
}

type blockingResponse struct {
	header          http.Header
	entered, resume chan struct{}
	once            sync.Once
}

func (w *blockingResponse) Header() http.Header { return w.header }
func (w *blockingResponse) WriteHeader(_ int)   {}
func (w *blockingResponse) Write(p []byte) (int, error) {
	w.once.Do(func() { close(w.entered) })
	<-w.resume
	return len(p), nil
}
func TestOversizedDiagnosticIsBounded(t *testing.T) {
	stock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user/login" {
			io.WriteString(w, `{"JWT Token":"owner"}`)
			return
		}
		io.WriteString(w, `{"body":"`+strings.Repeat("x", diagnosticReadSize+100)+`"}`)
	}))
	defer stock.Close()
	c := Controller{backend: stock.URL, client: stock.Client()}
	if _, err := c.stockDiagnostic(context.Background(), "syslog", "original"); err == nil {
		t.Fatal("unbounded report accepted")
	}
}
