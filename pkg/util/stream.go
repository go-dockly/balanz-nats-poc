package util

import (
	"context"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

func WaitForStream(ctx context.Context, js jetstream.JetStream, name string) (jetstream.Stream, error) {
	var lastErr error
	for i := 0; i < 30; i++ {
		stream, err := js.Stream(ctx, name)
		if err == nil {
			return stream, nil
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	return nil, lastErr
}
