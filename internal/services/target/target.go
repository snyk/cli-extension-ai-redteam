package target

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/jmespath/go-jmespath"
)

type Client interface {
	SendPrompt(ctx context.Context, prompt string) (string, error)
}

type HTTPClient struct {
	url                 string
	headers             map[string]string
	requestBodyTemplate string
	responseSelector    string
	httpClient          *http.Client
}

var _ Client = (*HTTPClient)(nil)

func NewHTTPClient(
	httpClient *http.Client,
	url string,
	headers map[string]string,
	requestBodyTemplate string,
	responseSelector string,
) *HTTPClient {
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	return &HTTPClient{
		url:                 url,
		headers:             headers,
		requestBodyTemplate: requestBodyTemplate,
		responseSelector:    responseSelector,
		httpClient:          httpClient,
	}
}

func (c *HTTPClient) SendPrompt(ctx context.Context, prompt string) (string, error) {
	body, err := buildRequestBody(c.requestBodyTemplate, prompt)
	if err != nil {
		return "", fmt.Errorf("build request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range c.headers {
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
		return "", fmt.Errorf("target returned status %d: %s", resp.StatusCode, string(respBytes))
	}

	return extractResponse(respBytes, c.responseSelector)
}

func buildRequestBody(template, prompt string) ([]byte, error) {
	escaped, err := json.Marshal(prompt)
	if err != nil {
		return nil, fmt.Errorf("json escape prompt: %w", err)
	}
	inner := string(escaped[1 : len(escaped)-1])
	result := strings.ReplaceAll(template, "{{prompt}}", inner)

	var check json.RawMessage
	if err := json.Unmarshal([]byte(result), &check); err != nil {
		return nil, fmt.Errorf("template produced invalid JSON: %w", err)
	}

	return []byte(result), nil
}

func extractResponse(respBytes []byte, selector string) (string, error) {
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
