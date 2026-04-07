package target

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"net/http"
	"strings"
	"time"

	"github.com/jmespath/go-jmespath"

	"github.com/snyk/cli-extension-ai-redteam/internal/utils"
)

const (
	DefaultTimeout        = 60 * time.Second
	maxRetries            = 3
	baseRetryDelay        = 1 * time.Second
	maxRetryDelay         = 15 * time.Second
	ConsecutiveFailureMax = 5
)

var ErrCircuitOpen = errors.New("target appears unreachable, aborting after too many consecutive failures")

type Client interface {
	SendPrompt(ctx context.Context, prompt string) (string, error)
	Ping(ctx context.Context) PingResult
}

// ClientOption configures optional behavior of an HTTPClient.
type ClientOption func(*HTTPClient)

// WithSessionStore configures the client to manage per-chat sessions.
func WithSessionStore(store *SessionStore) ClientOption {
	return func(c *HTTPClient) { c.sessionStore = store }
}

type HTTPClient struct {
	url                 string
	headers             map[string]string
	requestBodyTemplate string
	responseSelector    string
	httpClient          *http.Client
	consecutiveFailures int
	lastErr             error
	sessionStore        *SessionStore
}

var _ Client = (*HTTPClient)(nil)

func NewHTTPClient(
	httpClient *http.Client,
	url string,
	headers map[string]string,
	requestBodyTemplate string,
	responseSelector string,
	opts ...ClientOption,
) *HTTPClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: DefaultTimeout}
	}
	c := &HTTPClient{
		url:                 url,
		headers:             headers,
		requestBodyTemplate: requestBodyTemplate,
		responseSelector:    responseSelector,
		httpClient:          httpClient,
	}

	// This enables functional options pattern
	// e.g. NewHTTPClient(httpClient, ..., WithSessionStore(sessionStore))
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *HTTPClient) SendPrompt(ctx context.Context, prompt string) (string, error) {
	if c.consecutiveFailures >= ConsecutiveFailureMax {
		return "", fmt.Errorf("%w: last error: %w", ErrCircuitOpen, c.lastErr)
	}

	cc, _ := chatContextFrom(ctx)
	sessionID, err := c.sessionStore.GetOrCreate(ctx, cc.ChatID)
	if err != nil {
		return "", fmt.Errorf("resolve session: %w", err)
	}

	body, err := buildRequestBody(c.requestBodyTemplate, prompt, sessionID)
	if err != nil {
		return "", fmt.Errorf("build request body: %w", err)
	}

	var lastErr error

	for attempt := range maxRetries {
		result, err := c.doRequest(ctx, body, sessionID, cc.ChatID)
		if err == nil {
			c.consecutiveFailures = 0
			return result, nil
		}
		lastErr = err

		if !isRetryable(err) {
			c.consecutiveFailures++
			c.lastErr = err
			return "", err
		}

		if attempt < maxRetries-1 {
			select {
			case <-time.After(retryDelay(attempt)):
			case <-ctx.Done():
				return "", fmt.Errorf("target request canceled: %w", ctx.Err())
			}
		}
	}

	c.consecutiveFailures++
	c.lastErr = lastErr
	return "", fmt.Errorf("target request failed after %d attempts: %w", maxRetries, lastErr)
}

func (c *HTTPClient) doRequest(ctx context.Context, body []byte, sessionID, chatID string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	headers := resolveHeaders(c.headers, sessionID)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("target request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read target response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", &serverError{statusCode: resp.StatusCode, body: utils.TruncateBody(respBytes)}
	}

	c.sessionStore.OnResponse(chatID, resp.Header, respBytes)

	return extractResponse(respBytes, c.responseSelector)
}

type serverError struct {
	statusCode int
	body       string
}

func (e *serverError) Error() string {
	return fmt.Sprintf("target returned status %d: %s", e.statusCode, e.body)
}

func isRetryable(err error) bool {
	var se *serverError
	if errors.As(err, &se) {
		return true
	}
	if strings.Contains(err.Error(), "target request failed:") {
		return true
	}
	return false
}

// retryDelay returns an exponential backoff duration with jitter.
func retryDelay(attempt int) time.Duration {
	delay := baseRetryDelay * time.Duration(math.Pow(2, float64(attempt)))
	if delay > maxRetryDelay {
		delay = maxRetryDelay
	}
	jitter := time.Duration(rand.Int64N(int64(delay / 2))) //nolint:gosec // jitter doesn't need crypto rand
	return delay + jitter
}

func buildRequestBody(template, prompt, sessionID string) ([]byte, error) {
	escaped, err := json.Marshal(prompt)
	if err != nil {
		return nil, fmt.Errorf("json escape prompt: %w", err)
	}
	inner := string(escaped[1 : len(escaped)-1])
	result := strings.ReplaceAll(template, "{{prompt}}", inner)
	result = strings.ReplaceAll(result, "{{sessionId}}", sessionID)

	var check json.RawMessage
	if err := json.Unmarshal([]byte(result), &check); err != nil {
		return nil, fmt.Errorf("template produced invalid JSON: %w", err)
	}

	return []byte(result), nil
}

// resolveHeaders returns a copy of headers with {{sessionId}} replaced in values.
// Replacement always happens (even with empty sessionID) so that the literal
// placeholder is never sent over the wire.
func resolveHeaders(headers map[string]string, sessionID string) map[string]string {
	out := make(map[string]string, len(headers))
	for k, v := range headers {
		out[k] = strings.ReplaceAll(v, "{{sessionId}}", sessionID)
	}
	return out
}

func extractResponse(respBytes []byte, selector string) (string, error) {
	if selector == "" {
		return string(respBytes), nil
	}

	var data any
	if err := json.Unmarshal(respBytes, &data); err != nil {
		return "", fmt.Errorf("parse target response JSON: %w", err)
	}

	result, err := jmespath.Search(selector, data)
	if err != nil {
		return "", fmt.Errorf("response_selector %q: %w", selector, err)
	}
	if result == nil {
		return "", fmt.Errorf("response_selector %q: no match found", selector)
	}

	switch v := result.(type) {
	case string:
		return v, nil
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("marshal extracted value: %w", err)
		}
		return string(b), nil
	}
}
