package target

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"
)

const (
	PingMessage = "Hey, how are you?"
	PingTimeout = 20 * time.Second
)

type PingResult struct {
	Success       bool     `json:"success"`
	Response      string   `json:"response,omitempty"`
	Error         string   `json:"error,omitempty"`
	Suggestion    string   `json:"suggestion"`
	RawBody       string   `json:"raw_body,omitempty"`
	AvailableKeys []string `json:"available_keys,omitempty"`
}

func (c *HTTPClient) Ping(ctx context.Context) PingResult {
	body, err := buildRequestBody(c.requestBodyTemplate, PingMessage, "")
	if err != nil {
		return PingResult{
			Error:      err.Error(),
			Suggestion: "Request body template is invalid.",
		}
	}

	ctx, cancel := context.WithTimeout(ctx, PingTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return PingResult{
			Error:      err.Error(),
			Suggestion: "Failed to create request. Check the URL format.",
		}
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return classifyConnectionError(err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return PingResult{
			Error:      fmt.Sprintf("failed to read response: %s", err),
			Suggestion: "Target is reachable but the response could not be read.",
		}
	}

	if result, done := classifyStatusCode(resp.StatusCode, string(respBytes)); done {
		return result
	}

	return c.parseResponse(respBytes)
}

func classifyStatusCode(statusCode int, rawBody string) (PingResult, bool) {
	truncatedBody := truncate(rawBody, 500)

	if statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden {
		return PingResult{
			Error:      fmt.Sprintf("HTTP %d", statusCode),
			Suggestion: "Authentication failed. Check your headers (e.g. Authorization).",
			RawBody:    truncatedBody,
		}, true
	}
	if statusCode == http.StatusNotFound {
		return PingResult{
			Error:      "HTTP 404",
			Suggestion: "Endpoint not found. Verify the URL path.",
			RawBody:    truncatedBody,
		}, true
	}
	if statusCode >= 500 {
		return PingResult{
			Error:      fmt.Sprintf("HTTP %d", statusCode),
			Suggestion: fmt.Sprintf("Server error on the target side (status %d).", statusCode),
			RawBody:    truncatedBody,
		}, true
	}
	if statusCode < 200 || statusCode >= 300 {
		return PingResult{
			Error:      fmt.Sprintf("HTTP %d", statusCode),
			Suggestion: fmt.Sprintf("Target returned unexpected status %d.", statusCode),
			RawBody:    truncatedBody,
		}, true
	}

	return PingResult{}, false
}

func (c *HTTPClient) parseResponse(respBytes []byte) PingResult {
	rawBody := string(respBytes)

	// No selector configured — we can only verify reachability (2xx), not response structure.
	if c.responseSelector == "" {
		return PingResult{
			Success:    true,
			Response:   truncate(rawBody, 500),
			Suggestion: "Target is reachable and responding correctly.",
		}
	}

	if !json.Valid(respBytes) {
		return PingResult{
			Error:      "non-JSON response",
			Suggestion: "Target didn't return JSON. Verify the URL points to a JSON API endpoint.",
			RawBody:    truncate(rawBody, 500),
		}
	}

	extracted, err := extractResponse(respBytes, c.responseSelector)
	if err != nil {
		return c.buildSelectorErrorResult(respBytes, rawBody, err)
	}

	return PingResult{
		Success:    true,
		Response:   extracted,
		Suggestion: "Target is reachable and responding correctly.",
	}
}

func (c *HTTPClient) buildSelectorErrorResult(respBytes []byte, rawBody string, err error) PingResult {
	prettyBody := prettyJSON(respBytes, rawBody)
	if strings.Contains(err.Error(), "no match found") {
		var parsed any
		if unmarshalErr := json.Unmarshal(respBytes, &parsed); unmarshalErr != nil {
			return PingResult{
				Error:      err.Error(),
				Suggestion: fmt.Sprintf("Response selector %q failed: %s", c.responseSelector, unmarshalErr.Error()),
				RawBody:    prettyBody,
			}
		}
		paths := extractJMESPaths(parsed, "", 3)
		suggestion := fmt.Sprintf("Response selector %q didn't match.", c.responseSelector)
		if len(paths) > 0 {
			suggestion += fmt.Sprintf(" Available selectors: %s", strings.Join(paths, ", "))
		}
		return PingResult{
			Error:         err.Error(),
			Suggestion:    suggestion,
			RawBody:       prettyBody,
			AvailableKeys: paths,
		}
	}
	return PingResult{
		Error:      err.Error(),
		Suggestion: fmt.Sprintf("Response selector %q failed: %s", c.responseSelector, err.Error()),
		RawBody:    prettyBody,
	}
}

func classifyConnectionError(err error) PingResult {
	msg := err.Error()

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return PingResult{
			Error:      msg,
			Suggestion: "Target is unreachable. Check the URL and ensure the server is running.",
		}
	}

	if strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "dial tcp") {
		return PingResult{
			Error:      msg,
			Suggestion: "Target is unreachable. Check the URL and ensure the server is running.",
		}
	}

	return PingResult{
		Error:      msg,
		Suggestion: "Target request failed. Check the URL and network connectivity.",
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func prettyJSON(raw []byte, fallback string) string {
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return truncate(fallback, 500)
	}
	return truncate(buf.String(), 2000)
}

func extractJMESPaths(data any, prefix string, depth int) []string {
	if depth <= 0 {
		return nil
	}
	var paths []string
	switch v := data.(type) {
	case map[string]any:
		for key, val := range v {
			fullKey := key
			if prefix != "" {
				fullKey = prefix + "." + key
			}
			sub := extractJMESPaths(val, fullKey, depth-1)
			if len(sub) > 0 {
				paths = append(paths, sub...)
			} else {
				paths = append(paths, fullKey)
			}
		}
	case []any:
		if len(v) > 0 {
			arrayPath := prefix + "[0]"
			sub := extractJMESPaths(v[0], arrayPath, depth-1)
			if len(sub) > 0 {
				paths = append(paths, sub...)
			} else {
				paths = append(paths, arrayPath)
			}
		}
	default:
		if prefix != "" {
			return []string{prefix}
		}
	}
	sort.Strings(paths)
	return paths
}
