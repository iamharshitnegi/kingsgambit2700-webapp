package middleware

import "github.com/gofiber/fiber/v3"

// ConfigureTrustedProxies accepts forwarded client addresses only from the
// explicit proxy allowlist. An empty allowlist ignores all forwarded headers.
func ConfigureTrustedProxies(base fiber.Config, proxies []string) fiber.Config {
	base.TrustProxy = true
	base.TrustProxyConfig = fiber.TrustProxyConfig{Proxies: proxies}
	base.ProxyHeader = fiber.HeaderXForwardedFor
	base.EnableIPValidation = true
	return base
}
