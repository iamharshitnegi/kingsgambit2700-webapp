package broadcast

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
)

const channel = "kg:classroom:invalidate:v1"

type Redis struct{ client *redis.Client }

func NewRedis(rawURL string) (*Redis, error) {
	options, err := redis.ParseURL(rawURL)
	if err != nil {
		return nil, fmt.Errorf("broadcast: parsing redis url: %w", err)
	}
	return &Redis{client: redis.NewClient(options)}, nil
}

func (r *Redis) Ping(ctx context.Context) error {
	if err := r.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("broadcast: pinging redis: %w", err)
	}
	return nil
}

func (r *Redis) Publish(ctx context.Context, notification Notification) error {
	if notification.SessionID == "" || notification.Revision == 0 {
		return ErrInvalidNotification
	}
	payload, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("broadcast: encoding notification: %w", err)
	}
	if err := r.client.Publish(ctx, channel, payload).Err(); err != nil {
		return fmt.Errorf("broadcast: publishing notification: %w", err)
	}
	return nil
}

func (r *Redis) Subscribe(ctx context.Context, handle func(context.Context, Notification) error) error {
	if handle == nil {
		return errors.New("broadcast: subscriber callback is required")
	}
	subscription := r.client.Subscribe(ctx, channel)
	defer subscription.Close()
	if _, err := subscription.Receive(ctx); err != nil {
		return fmt.Errorf("broadcast: subscribing: %w", err)
	}
	for {
		message, err := subscription.ReceiveMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("broadcast: receiving notification: %w", err)
		}
		var notification Notification
		if err := json.Unmarshal([]byte(message.Payload), &notification); err != nil || notification.SessionID == "" || notification.Revision == 0 {
			continue
		}
		if err := handle(ctx, notification); err != nil {
			return err
		}
	}
}

func (r *Redis) Close() error { return r.client.Close() }
