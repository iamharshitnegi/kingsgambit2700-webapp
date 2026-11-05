package metrics

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Registry struct {
	requests sync.Map
	inFlight atomic.Int64
}

type requestMetric struct {
	count   atomic.Uint64
	errors  atomic.Uint64
	totalNS atomic.Uint64
}

func New() *Registry { return &Registry{} }

func (r *Registry) Observe(method, route string, status int, duration time.Duration) {
	key := sanitize(method) + `|` + sanitize(route)
	value, _ := r.requests.LoadOrStore(key, &requestMetric{})
	metric := value.(*requestMetric)
	metric.count.Add(1)
	if status >= 500 {
		metric.errors.Add(1)
	}
	metric.totalNS.Add(uint64(duration))
}

func (r *Registry) IncInFlight() { r.inFlight.Add(1) }
func (r *Registry) DecInFlight() { r.inFlight.Add(-1) }

func (r *Registry) WritePrometheus(w io.Writer) error {
	if _, err := fmt.Fprintln(w, "# TYPE kg_http_in_flight gauge"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "kg_http_in_flight %d\n", r.inFlight.Load()); err != nil {
		return err
	}

	keys := make([]string, 0)
	r.requests.Range(func(key, _ any) bool {
		keys = append(keys, key.(string))
		return true
	})
	sort.Strings(keys)

	for _, key := range keys {
		value, _ := r.requests.Load(key)
		metric := value.(*requestMetric)
		parts := strings.SplitN(key, "|", 2)
		labels := fmt.Sprintf(`method=%q,route=%q`, parts[0], parts[1])
		if _, err := fmt.Fprintf(w, "kg_http_requests_total{%s} %d\n", labels, metric.count.Load()); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "kg_http_errors_total{%s} %d\n", labels, metric.errors.Load()); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "kg_http_request_duration_seconds_sum{%s} %.6f\n", labels, float64(metric.totalNS.Load())/float64(time.Second)); err != nil {
			return err
		}
	}
	return nil
}

func sanitize(value string) string {
	value = strings.ReplaceAll(value, `\\`, `_`)
	value = strings.ReplaceAll(value, `"`, `_`)
	value = strings.ReplaceAll(value, "\n", `_`)
	return value
}
