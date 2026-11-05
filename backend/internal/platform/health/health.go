package health

import (
	"context"
	"errors"
	"sync"
	"time"
)

type Check func(context.Context) error

type Result struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks,omitempty"`
}

type Checker struct {
	mu     sync.RWMutex
	checks map[string]Check
}

func New() *Checker {
	return &Checker{checks: make(map[string]Check)}
}

func (c *Checker) Add(name string, check Check) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks[name] = check
}

func (c *Checker) Ready(ctx context.Context) (Result, error) {
	c.mu.RLock()
	checks := make(map[string]Check, len(c.checks))
	for name, check := range c.checks {
		checks[name] = check
	}
	c.mu.RUnlock()

	result := Result{Status: "ok", Checks: make(map[string]string, len(checks))}
	var failures []error
	for name, check := range checks {
		checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err := check(checkCtx)
		cancel()
		if err != nil {
			result.Status = "unavailable"
			result.Checks[name] = "failed"
			failures = append(failures, err)
			continue
		}
		result.Checks[name] = "ok"
	}
	return result, errors.Join(failures...)
}
