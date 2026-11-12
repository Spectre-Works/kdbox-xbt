package main

import (
	"io"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

// Keep both runtime and preflight logs in RAM at a fixed budget. Truncating
// the same inode preserves the supervisor's stderr descriptor across rotation.
// Never load an old, potentially oversized log in full.
type boundedLog struct {
	mu      sync.Mutex
	path    string
	maximum int64
}

func (l *boundedLog) Write(data []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	original := len(data)
	if int64(len(data)) > l.maximum {
		data = data[int64(len(data))-l.maximum:]
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0600)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return 0, err
	}
	if info.Size()+int64(len(data)) > l.maximum {
		length := min(info.Size(), l.maximum)
		tail, err := io.ReadAll(io.NewSectionReader(f, info.Size()-length, length))
		if err != nil {
			return 0, err
		}
		if err = os.WriteFile(l.path+".1", tail, 0600); err != nil {
			return 0, err
		}
		if err = f.Truncate(0); err != nil {
			return 0, err
		}
	}
	if _, err = f.Write(data); err != nil {
		return 0, err
	}
	return original, nil
}

// No fsync is needed for this RAM-backed liveness marker. Rename prevents a
// watchdog read from observing the empty interval of a truncate-and-write.
func writeHeartbeat(path string, now time.Time) error {
	if err := os.WriteFile(path+".new", []byte(strconv.FormatInt(now.Unix(), 10)), 0600); err != nil {
		return err
	}
	return os.Rename(path+".new", path)
}

type boundedListener struct {
	net.Listener
	slots chan struct{}
	done  chan struct{}
	once  sync.Once
}

func newBoundedListener(listener net.Listener, maximum int) *boundedListener {
	return &boundedListener{Listener: listener, slots: make(chan struct{}, maximum), done: make(chan struct{})}
}
func (l *boundedListener) Accept() (net.Conn, error) {
	select {
	case l.slots <- struct{}{}:
	case <-l.done:
		return nil, net.ErrClosed
	}
	c, err := l.Listener.Accept()
	if err != nil {
		<-l.slots
		return nil, err
	}
	return &boundedConnection{Conn: c, release: func() { <-l.slots }}, nil
}
func (l *boundedListener) Close() error {
	var err error
	l.once.Do(func() { close(l.done); err = l.Listener.Close() })
	return err
}

type boundedConnection struct {
	net.Conn
	once    sync.Once
	release func()
}

func (c *boundedConnection) Close() error {
	err := c.Conn.Close()
	c.once.Do(c.release)
	return err
}
