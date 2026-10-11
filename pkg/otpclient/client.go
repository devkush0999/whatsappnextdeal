// Package otpclient calls Evolution's OTP API from a trusted application backend.
package otpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL, key string
	http         *http.Client
}

func New(baseURL, apiKey string) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && u.Scheme != "http") {
		return nil, fmt.Errorf("invalid OTP base URL")
	}
	if !strings.HasPrefix(apiKey, "otp_") {
		return nil, fmt.Errorf("invalid project API key format")
	}
	return &Client{strings.TrimRight(baseURL, "/"), apiKey, &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

type Request struct {
	Phone     string `json:"phone"`
	Purpose   string `json:"purpose"`
	Reference string `json:"reference"`
	Consent   bool   `json:"consent"`
}
type Challenge struct {
	ID             string    `json:"challengeId"`
	Status         string    `json:"status"`
	DeliveryStatus string    `json:"deliveryStatus"`
	Destination    string    `json:"destination"`
	ExpiresAt      time.Time `json:"expiresAt"`
	ResendAfter    time.Time `json:"resendAfter"`
	Sandbox        bool      `json:"sandbox"`
}
type Verify struct {
	ChallengeID string `json:"challengeId"`
	Code        string `json:"code"`
	Purpose     string `json:"purpose"`
	Reference   string `json:"reference"`
}
type Verification struct {
	Verified    bool   `json:"verified"`
	ChallengeID string `json:"challengeId"`
	Phone       string `json:"phone"`
	Purpose     string `json:"purpose"`
	Reference   string `json:"reference"`
	Sandbox     bool   `json:"sandbox"`
}
type APIError struct {
	Status     int    `json:"-"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	RetryAfter int    `json:"retryAfter"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("OTP API %d %s: %s", e.Status, e.Code, e.Message)
}
func (c *Client) call(ctx context.Context, method, path, idempotency string, in, out interface{}) error {
	var body []byte
	var err error
	if in != nil {
		body, err = json.Marshal(in)
		if err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Content-Type", "application/json")
	if idempotency != "" {
		req.Header.Set("Idempotency-Key", idempotency)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	reader := io.LimitReader(resp.Body, 65536)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var data struct {
			Error APIError `json:"error"`
		}
		if json.NewDecoder(reader).Decode(&data) != nil {
			return &APIError{Status: resp.StatusCode, Code: "INVALID_RESPONSE", Message: "Unexpected provider response"}
		}
		data.Error.Status = resp.StatusCode
		return &data.Error
	}
	return json.NewDecoder(reader).Decode(out)
}
func (c *Client) Request(ctx context.Context, in Request, idempotency string) (Challenge, error) {
	var out Challenge
	if idempotency == "" {
		return out, fmt.Errorf("an idempotency key is required")
	}
	err := c.call(ctx, "POST", "/v1/otp/request", idempotency, in, &out)
	return out, err
}
func (c *Client) Resend(ctx context.Context, id string, consent bool, idempotency string) (Challenge, error) {
	var out Challenge
	if idempotency == "" {
		return out, fmt.Errorf("an idempotency key is required")
	}
	err := c.call(ctx, "POST", "/v1/otp/resend", idempotency, map[string]interface{}{"challengeId": id, "consent": consent}, &out)
	return out, err
}
func (c *Client) Verify(ctx context.Context, in Verify) (Verification, error) {
	var out Verification
	err := c.call(ctx, "POST", "/v1/otp/verify", "", in, &out)
	return out, err
}
func (c *Client) Status(ctx context.Context, id string) (Challenge, error) {
	var out Challenge
	err := c.call(ctx, "GET", "/v1/otp/challenges/"+url.PathEscape(id), "", nil, &out)
	return out, err
}
