package smtp

import (
	"context"
	"errors"
	"fmt"
	stdsmtp "net/smtp"
	"strconv"
	"strings"

	"github.com/kingsgambit2700/platform/backend/internal/provider"
)

type Client struct {
	host     string
	port     int
	username string
	password string
	from     string
}

func New(host string, port int, username, password, from string) (*Client, error) {
	if host == "" || port <= 0 || from == "" {
		return nil, errors.New("smtp: host, positive port, and from address are required")
	}
	return &Client{host: host, port: port, username: username, password: password, from: from}, nil
}

func (c *Client) Send(ctx context.Context, message provider.EmailMessage) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	var auth stdsmtp.Auth
	if c.username != "" {
		auth = stdsmtp.PlainAuth("", c.username, c.password, c.host)
	}
	headers := []string{
		"From: " + c.from,
		"To: " + message.To,
		"Subject: " + message.Subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		message.TextBody,
	}
	address := c.host + ":" + strconv.Itoa(c.port)
	if err := stdsmtp.SendMail(address, auth, c.from, []string{message.To}, []byte(strings.Join(headers, "\r\n"))); err != nil {
		return "", fmt.Errorf("smtp: sending mail: %w", err)
	}
	return "smtp", nil
}
