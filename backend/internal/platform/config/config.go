package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	GoogleClientID        string
	Environment           string
	APIAddr               string
	RealtimeAddr          string
	DatabaseURL           string
	DatabaseDirectURL     string
	RedisURL              string
	PublicAPIURL          string
	PublicWebURL          string
	AllowedOrigins        []string
	TrustedProxies        []string
	CookieName            string
	CookieSecure          bool
	SessionIdleTTL        time.Duration
	SessionAbsoluteTTL    time.Duration
	MagicLinkTTL          time.Duration
	CSRFTTL               time.Duration
	ResendAPIKey          string
	ResendFrom            string
	DailyAPIKey           string
	DailyBaseURL          string
	RazorpayKeyID         string
	RazorpayKeySecret     string
	RazorpayWebhookSecret string
	ResendWebhookSecret   string
	DailyWebhookSecret    string
	TOTPEncryptionKey     string
	MFAChallengeTTL       time.Duration
	MFARecentTTL          time.Duration
	EmailProvider         string
	SMTPHost              string
	SMTPPort              int
	SMTPUser              string
	SMTPPassword          string
	SMTPFrom              string
	S3Endpoint            string
	S3Region              string
	S3BucketClean         string
	S3BucketQuarantine    string
	S3AccessKeyID         string
	S3SecretAccessKey     string
	S3ForcePathStyle      bool
	ClamAVAddr            string
	UploadMaxBytes        int64
	OTELEndpoint          string
	OTELServiceName       string
	LogLevel              string
}

func Load() (Config, error) {
	environment := value("KG_ENV", "development")
	defaultCookieName := "kg_session"
	defaultEmailProvider := "fake"
	if environment != "development" && environment != "test" {
		defaultCookieName = "__Host-kg_session"
		defaultEmailProvider = "resend"
	}
	cfg := Config{
		GoogleClientID:        strings.TrimSpace(os.Getenv("KG_GOOGLE_CLIENT_ID")),
		Environment:           environment,
		APIAddr:               value("KG_API_ADDR", ":8080"),
		RealtimeAddr:          value("KG_REALTIME_ADDR", ":8081"),
		DatabaseURL:           os.Getenv("KG_DATABASE_URL"),
		DatabaseDirectURL:     os.Getenv("KG_DATABASE_DIRECT_URL"),
		RedisURL:              value("KG_REDIS_URL", "redis://localhost:6379/0"),
		PublicAPIURL:          value("KG_PUBLIC_API_URL", "http://localhost:8080"),
		PublicWebURL:          value("KG_PUBLIC_WEB_URL", "http://localhost:3000"),
		AllowedOrigins:        splitCSV(value("KG_ALLOWED_ORIGINS", "http://localhost:3000")),
		TrustedProxies:        splitCSV(os.Getenv("KG_TRUSTED_PROXIES")),
		CookieName:            value("KG_COOKIE_NAME", defaultCookieName),
		ResendAPIKey:          os.Getenv("KG_RESEND_API_KEY"),
		ResendFrom:            value("KG_RESEND_FROM", "academy@example.test"),
		DailyAPIKey:           os.Getenv("KG_DAILY_API_KEY"),
		DailyBaseURL:          value("KG_DAILY_BASE_URL", "https://api.daily.co/v1"),
		RazorpayKeyID:         os.Getenv("KG_RAZORPAY_KEY_ID"),
		RazorpayKeySecret:     os.Getenv("KG_RAZORPAY_KEY_SECRET"),
		RazorpayWebhookSecret: os.Getenv("KG_RAZORPAY_WEBHOOK_SECRET"),
		ResendWebhookSecret:   os.Getenv("KG_RESEND_WEBHOOK_SECRET"),
		DailyWebhookSecret:    os.Getenv("KG_DAILY_WEBHOOK_SECRET"),
		TOTPEncryptionKey:     os.Getenv("KG_TOTP_ENCRYPTION_KEY"),
		EmailProvider:         strings.ToLower(value("KG_EMAIL_PROVIDER", defaultEmailProvider)),
		SMTPHost:              os.Getenv("KG_SMTP_HOST"),
		SMTPUser:              os.Getenv("KG_SMTP_USER"),
		SMTPPassword:          os.Getenv("KG_SMTP_PASSWORD"),
		SMTPFrom:              value("KG_SMTP_FROM", "academy@example.test"),
		S3Endpoint:            os.Getenv("KG_S3_ENDPOINT"),
		S3Region:              value("KG_S3_REGION", "ap-south-1"),
		S3BucketClean:         os.Getenv("KG_S3_BUCKET_CLEAN"),
		S3BucketQuarantine:    os.Getenv("KG_S3_BUCKET_QUARANTINE"),
		S3AccessKeyID:         os.Getenv("KG_S3_ACCESS_KEY_ID"),
		S3SecretAccessKey:     os.Getenv("KG_S3_SECRET_ACCESS_KEY"),
		ClamAVAddr:            value("KG_CLAMAV_ADDR", "localhost:3310"),
		OTELEndpoint:          os.Getenv("KG_OTEL_EXPORTER_OTLP_ENDPOINT"),
		OTELServiceName:       value("KG_OTEL_SERVICE_NAME", "kingsgambit2700-backend"),
		LogLevel:              value("KG_LOG_LEVEL", "info"),
	}

	var err error
	if cfg.CookieSecure, err = parseBool("KG_COOKIE_SECURE", cfg.Environment != "development"); err != nil {
		return Config{}, err
	}
	if cfg.SessionIdleTTL, err = parseDuration("KG_SESSION_IDLE_TTL", 30*time.Minute); err != nil {
		return Config{}, err
	}
	if cfg.SessionAbsoluteTTL, err = parseDuration("KG_SESSION_ABSOLUTE_TTL", 7*24*time.Hour); err != nil {
		return Config{}, err
	}
	if cfg.MagicLinkTTL, err = parseDuration("KG_MAGIC_LINK_TTL", 15*time.Minute); err != nil {
		return Config{}, err
	}
	if cfg.CSRFTTL, err = parseDuration("KG_CSRF_TTL", 30*time.Minute); err != nil {
		return Config{}, err
	}
	if cfg.MFAChallengeTTL, err = parseDuration("KG_MFA_CHALLENGE_TTL", 5*time.Minute); err != nil {
		return Config{}, err
	}
	if cfg.MFARecentTTL, err = parseDuration("KG_MFA_RECENT_TTL", 5*time.Minute); err != nil {
		return Config{}, err
	}
	if cfg.SMTPPort, err = parseInt("KG_SMTP_PORT", 587); err != nil {
		return Config{}, err
	}
	if cfg.UploadMaxBytes, err = parseInt64("KG_UPLOAD_MAX_BYTES", 25<<20); err != nil {
		return Config{}, err
	}
	if cfg.S3ForcePathStyle, err = parseBool("KG_S3_FORCE_PATH_STYLE", cfg.Environment == "development"); err != nil {
		return Config{}, err
	}

	if cfg.DatabaseDirectURL == "" {
		cfg.DatabaseDirectURL = cfg.DatabaseURL
	}
	if len(cfg.AllowedOrigins) == 0 {
		return Config{}, errors.New("config: at least one allowed origin is required")
	}
	for _, proxy := range cfg.TrustedProxies {
		if _, err := netip.ParseAddr(proxy); err == nil {
			continue
		}
		if _, err := netip.ParsePrefix(proxy); err != nil {
			return Config{}, fmt.Errorf("config: KG_TRUSTED_PROXIES requires IP addresses or CIDRs, got %q", proxy)
		}
	}
	for _, origin := range cfg.AllowedOrigins {
		if origin == "*" {
			return Config{}, errors.New("config: wildcard origin is forbidden with credentialed requests")
		}
		if err := validateOrigin(origin); err != nil {
			return Config{}, fmt.Errorf("config: invalid KG_ALLOWED_ORIGINS value %q: %w", origin, err)
		}
	}
	if err := validateOrigin(cfg.PublicWebURL); err != nil {
		return Config{}, fmt.Errorf("config: invalid KG_PUBLIC_WEB_URL: %w", err)
	}
	if err := validateOrigin(cfg.PublicAPIURL); err != nil {
		return Config{}, fmt.Errorf("config: invalid KG_PUBLIC_API_URL: %w", err)
	}
	if cfg.CookieName == "__Host-kg_session" && !cfg.CookieSecure {
		return Config{}, errors.New("config: __Host- cookie requires KG_COOKIE_SECURE=true")
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("config: KG_DATABASE_URL is required")
	}
	if cfg.TOTPEncryptionKey != "" {
		if err := validateTOTPEncryptionKey(cfg.TOTPEncryptionKey); err != nil {
			return Config{}, err
		}
	}
	switch cfg.EmailProvider {
	case "fake":
		if cfg.Environment != "development" && cfg.Environment != "test" {
			return Config{}, errors.New("config: fake email provider is forbidden outside development/test")
		}
	case "resend":
		if cfg.ResendAPIKey == "" {
			return Config{}, errors.New("config: KG_RESEND_API_KEY is required for resend")
		}
	case "smtp":
		if cfg.SMTPHost == "" || cfg.SMTPFrom == "" {
			return Config{}, errors.New("config: KG_SMTP_HOST and KG_SMTP_FROM are required for smtp")
		}
	default:
		return Config{}, fmt.Errorf("config: unsupported KG_EMAIL_PROVIDER %q", cfg.EmailProvider)
	}
	if cfg.Environment == "production" {
		if cfg.CookieName != "__Host-kg_session" || !cfg.CookieSecure {
			return Config{}, errors.New("config: production requires a Secure __Host-kg_session cookie")
		}
		if cfg.TOTPEncryptionKey == "" || cfg.ResendWebhookSecret == "" || cfg.DailyWebhookSecret == "" || cfg.RazorpayWebhookSecret == "" {
			return Config{}, errors.New("config: production auth and provider webhook secrets are required")
		}
		if cfg.S3BucketClean == "" || cfg.S3BucketQuarantine == "" || cfg.ClamAVAddr == "" {
			return Config{}, errors.New("config: production object storage and malware scanning settings are required")
		}
	}
	return cfg, nil
}

func validateTOTPEncryptionKey(raw string) error {
	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	for _, encoding := range encodings {
		decoded, err := encoding.DecodeString(raw)
		if err == nil && len(decoded) == 32 {
			return nil
		}
	}
	return errors.New("config: KG_TOTP_ENCRYPTION_KEY must be base64-encoded 32-byte key material")
}

func value(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func splitCSV(raw string) []string {
	values := make([]string, 0)
	for _, part := range strings.Split(raw, ",") {
		if part = strings.TrimSpace(part); part != "" {
			values = append(values, part)
		}
	}
	return values
}

func validateOrigin(raw string) error {
	if strings.HasSuffix(raw, "/") {
		return errors.New("trailing slash is not allowed")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return errors.New("an absolute http(s) origin is required")
	}
	if parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("origin must not contain userinfo, path, query, or fragment")
	}
	if raw != parsed.Scheme+"://"+parsed.Host {
		return errors.New("origin is not canonical")
	}
	return nil
}

func parseBool(key string, fallback bool) (bool, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("config: parsing %s: %w", key, err)
	}
	return v, nil
}

func parseDuration(key string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	v, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("config: parsing %s: %w", key, err)
	}
	if v <= 0 {
		return 0, fmt.Errorf("config: %s must be positive", key)
	}
	return v, nil
}

func parseInt(key string, fallback int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("config: %s must be a positive integer", key)
	}
	return value, nil
}

func parseInt64(key string, fallback int64) (int64, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("config: %s must be a positive integer", key)
	}
	return value, nil
}
