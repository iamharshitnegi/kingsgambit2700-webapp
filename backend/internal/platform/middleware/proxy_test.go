package middleware

import (
	"net"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/valyala/fasthttp"
)

func TestForwardedClientAddress(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		proxies []string
		peer    string
		header  string
		want    string
	}{
		{name: "trusted proxy preserves visitor", proxies: []string{"172.30.27.10"}, peer: "172.30.27.10", header: "198.51.100.20", want: "198.51.100.20"},
		{name: "second visitor stays separate", proxies: []string{"172.30.27.10"}, peer: "172.30.27.10", header: "198.51.100.21", want: "198.51.100.21"},
		{name: "untrusted peer cannot spoof", proxies: []string{"172.30.27.10"}, peer: "192.0.2.10", header: "198.51.100.20", want: "192.0.2.10"},
		{name: "default ignores forwarding", peer: "192.0.2.10", header: "198.51.100.20", want: "192.0.2.10"},
		{name: "invalid address falls back", proxies: []string{"172.30.27.10"}, peer: "172.30.27.10", header: "invalid", want: "172.30.27.10"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New(ConfigureTrustedProxies(fiber.Config{}, test.proxies))
			var req fasthttp.Request
			req.Header.Set(fiber.HeaderXForwardedFor, test.header)
			var requestContext fasthttp.RequestCtx
			requestContext.Init(&req, &net.TCPAddr{IP: net.ParseIP(test.peer), Port: 12345}, nil)
			ctx := app.AcquireCtx(&requestContext)
			defer app.ReleaseCtx(ctx)
			if got := ctx.IP(); got != test.want {
				t.Fatalf("client IP = %q, want %q", got, test.want)
			}
		})
	}
}
