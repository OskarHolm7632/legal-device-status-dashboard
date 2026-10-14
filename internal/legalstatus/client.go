package legalstatus

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

const (
	createChannelPath = "/v1/realtime/channel/create"
	publishPath       = "/v1/realtime/publish"
	metricsQueryPath  = "/v1/metrics/query"
)

type APIError struct {
	Code       string
	Message    string
	HTTPStatus int
}

func (e *APIError) Error() string {
	if e.Code == "" {
		return e.Message
	}
	return e.Code + ": " + e.Message
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *apiErrorBody   `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type apiErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Client struct {
	baseURL string
	key     string
	http    *http.Client
	sleep   func(context.Context, time.Duration) error
}

func NewClient(baseURL, key string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		key:     key,
		http:    httpClient,
		sleep: func(ctx context.Context, d time.Duration) error {
			select {
			case <-time.After(d):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	}
}

func (c *Client) CreateChannel(ctx context.Context, channel, requestID string) error {
	body := struct {
		Channel string `json:"channel"`
		Type    string `json:"type"`
		Vendor  string `json:"vendor"`
	}{Channel: channel, Type: "device-status", Vendor: "legal-operations"}
	_, err := c.call(ctx, http.MethodPost, createChannelPath, body, requestID)
	return err
}

func (c *Client) Publish(ctx context.Context, channel, requestID string, decision Decision) error {
	body := struct {
		Channel string   `json:"channel"`
		Event   string   `json:"event"`
		Data    Decision `json:"data"`
	}{Channel: channel, Event: "matter.status.changed", Data: decision}
	_, err := c.call(ctx, http.MethodPost, publishPath, body, requestID)
	return err
}

func (c *Client) QueryMetrics(ctx context.Context) (json.RawMessage, error) {
	query := url.Values{
		"name": {"realtime.publish"},
		"agg":  {"count"},
	}
	return c.call(ctx, http.MethodGet, metricsQueryPath+"?"+query.Encode(), nil, "")
}

func (c *Client) call(ctx context.Context, method, path string, body any, idempotencyKey string) (json.RawMessage, error) {
	var encoded []byte
	var err error
	if body != nil {
		encoded, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}

	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(encoded))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if idempotencyKey != "" {
			req.Header.Set("Idempotency-Key", idempotencyKey)
		}

		res, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		raw, readErr := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		res.Body.Close()
		if readErr != nil {
			return nil, readErr
		}

		var env envelope
		decodeErr := json.Unmarshal(raw, &env)
		if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			delay := retryDelay(res.Header.Get("Retry-After"), attempt)
			if err := c.sleep(ctx, delay); err != nil {
				return nil, err
			}
			continue
		}
		if decodeErr != nil {
			return nil, fmt.Errorf("decode Infrai envelope: %w", decodeErr)
		}
		if !env.OK {
			apiErr := &APIError{HTTPStatus: res.StatusCode, Message: "Infrai request rejected"}
			if env.Error != nil {
				apiErr.Code = env.Error.Code
				apiErr.Message = env.Error.Message
			}
			return nil, apiErr
		}
		if res.StatusCode >= http.StatusInternalServerError {
			return nil, fmt.Errorf("Infrai transport status %d", res.StatusCode)
		}
		return env.Data, nil
	}
	return nil, errors.New("retry budget exhausted")
}

func retryDelay(value string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 250 * time.Millisecond
}
