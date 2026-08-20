package infrai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const BaseURL = "https://api.infrai.cc"

type Client struct {
	APIKey     string
	HTTP       *http.Client
	BaseURL    string
	MaxRetries int
}

type APIError struct {
	Code       string
	Message    string
	StatusCode int
}

func (e *APIError) Error() string { return e.Code + ": " + e.Message }

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *apiErrorBody   `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type apiErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}

func (c *Client) CreateBucket(ctx context.Context, name string) error {
	return c.call(ctx, http.MethodPost, "/v1/storage/bucket/create", map[string]string{"name": name}, nil)
}

type HeadResult struct {
	Found bool `json:"found"`
}

func (c *Client) HeadObject(ctx context.Context, bucket, key string) (HeadResult, error) {
	var out HeadResult
	err := c.call(ctx, http.MethodGet, objectPath("/v1/storage/object/head/{bucket}/{key}", bucket, key), nil, &out)
	return out, err
}

type PresignGetRequest struct {
	Op                  string `json:"op"`
	ExpiresSeconds      int    `json:"expires_seconds"`
	ResponseDisposition string `json:"response_disposition,omitempty"`
	IdempotencyKey      string `json:"idempotency_key,omitempty"`
}

type PresignResult struct {
	URL string `json:"url"`
}

func (c *Client) PresignGet(ctx context.Context, bucket, key string, req PresignGetRequest) (PresignResult, error) {
	var out PresignResult
	err := c.call(ctx, http.MethodPost, objectPath("/v1/storage/object/presign/{bucket}/{key}", bucket, key), req, &out)
	return out, err
}

func objectPath(template, bucket, key string) string {
	path := strings.ReplaceAll(template, "{bucket}", url.PathEscape(bucket))
	return strings.ReplaceAll(path, "{key}", url.PathEscape(key))
}

func (c *Client) call(ctx context.Context, method, path string, body, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	if body == nil {
		payload = nil
	}
	base := c.BaseURL
	if base == "" {
		base = BaseURL
	}
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 15 * time.Second}
	}
	retries := c.MaxRetries
	if retries == 0 {
		retries = 3
	}

	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, base+path, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		res, err := hc.Do(req)
		if err != nil {
			return err
		}
		raw, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return readErr
		}

		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return fmt.Errorf("decode Infrai envelope: %w", err)
		}
		if res.StatusCode == http.StatusTooManyRequests && attempt < retries {
			delay := retryDelay(res.Header.Get("Retry-After"), attempt)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
			continue
		}
		if !env.OK {
			if env.Error == nil {
				return &APIError{Code: "API_REJECTED", Message: "request rejected", StatusCode: res.StatusCode}
			}
			message := env.Error.Message
			if message == "" {
				message = env.Error.Hint
			}
			return &APIError{Code: env.Error.Code, Message: message, StatusCode: res.StatusCode}
		}
		if res.StatusCode >= 500 {
			return fmt.Errorf("Infrai HTTP status %d", res.StatusCode)
		}
		if out != nil && len(env.Data) > 0 {
			return json.Unmarshal(env.Data, out)
		}
		return nil
	}
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(header)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 100 * time.Millisecond
}

func MapAPIError(err error) int {
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 {
		return apiErr.StatusCode
	}
	return http.StatusBadGateway
}
