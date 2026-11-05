package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Decision struct {
	Allowed    bool
	RetryAfter time.Duration
}

type Limiter interface {
	Take(context.Context, string, int64, time.Duration) (Decision, error)
	Ping(context.Context) error
}

type Redis struct {
	client *redis.Client
}

func NewRedis(rawURL string) (*Redis, error) {
	options, err := redis.ParseURL(rawURL)
	if err != nil {
		return nil, fmt.Errorf("rate limit: parsing redis url: %w", err)
	}
	return &Redis{client: redis.NewClient(options)}, nil
}

var takeScript = redis.NewScript(`
local current = redis.call("INCR", KEYS[1])
if current == 1 then
  redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
local ttl = redis.call("PTTL", KEYS[1])
if ttl < 0 then
  redis.call("PEXPIRE", KEYS[1], ARGV[1])
  ttl = tonumber(ARGV[1])
end
return {current, ttl}
`)

func (r *Redis) Take(
	ctx context.Context,
	key string,
	limit int64,
	window time.Duration,
) (Decision, error) {
	if r == nil || r.client == nil {
		return Decision{}, errors.New("rate limit: redis client is required")
	}
	if key == "" || limit <= 0 || window <= 0 {
		return Decision{}, errors.New("rate limit: key, positive limit and window are required")
	}
	result, err := takeScript.Run(
		ctx,
		r.client,
		[]string{"kg:rate:v1:" + key},
		window.Milliseconds(),
	).Slice()
	if err != nil {
		return Decision{}, fmt.Errorf("rate limit: taking token: %w", err)
	}
	if len(result) != 2 {
		return Decision{}, errors.New("rate limit: invalid Redis response")
	}
	count, ok := result[0].(int64)
	if !ok {
		return Decision{}, errors.New("rate limit: invalid counter response")
	}
	ttlMillis, ok := result[1].(int64)
	if !ok {
		return Decision{}, errors.New("rate limit: invalid ttl response")
	}
	return Decision{
		Allowed:    count <= limit,
		RetryAfter: max(time.Duration(ttlMillis)*time.Millisecond, time.Second),
	}, nil
}

func (r *Redis) Ping(ctx context.Context) error {
	if err := r.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("rate limit: pinging redis: %w", err)
	}
	return nil
}

func (r *Redis) Close() error { return r.client.Close() }
