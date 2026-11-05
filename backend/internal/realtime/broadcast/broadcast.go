package broadcast

import (
	"context"
	"errors"
)

var ErrInvalidNotification = errors.New("broadcast: invalid notification")

type Notification struct {
	SessionID string `json:"sessionId"`
	Revision  uint64 `json:"revision"`
}

// Bus transports invalidation hints only. PostgreSQL remains the source of
// truth for board snapshots, events, authorization and accepted revisions.
type Bus interface {
	Publish(context.Context, Notification) error
	Subscribe(context.Context, func(context.Context, Notification) error) error
	Ping(context.Context) error
	Close() error
}
