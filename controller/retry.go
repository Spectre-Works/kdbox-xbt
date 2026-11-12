package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

type stockHTTPError int

func (e stockHTTPError) Error() string { return fmt.Sprintf("stock controller HTTP %d", int(e)) }

func retryRead(method, path string, err error) bool {
	if method != "GET" {
		return false
	}
	switch path {
	case "/mcb/status", "/mcb/setting", "/mcb/algosetting", "/mcb/cgminer?cgminercmd=devs", "/dbg/icinfo", "/cpb/hshistory":
	default:
		return false
	}
	var status stockHTTPError
	if errors.As(err, &status) {
		return status == 502 || status == 503 || status == 504
	}
	var network net.Error
	return errors.As(err, &network) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
}

func (c *Controller) native(method, path string, body any, auth bool) ([]byte, error) {
	return c.nativeContext(context.Background(), method, path, body, auth)
}

func (c *Controller) nativeContext(ctx context.Context, method, path string, body any, auth bool) ([]byte, error) {
	b, err := c.nativeOnceContext(ctx, method, path, body, auth)
	if err != nil && retryRead(method, path, err) {
		timer := time.NewTimer(150 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
		}
		return c.nativeOnceContext(ctx, method, path, body, auth)
	}
	return b, err
}
