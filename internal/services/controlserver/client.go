package controlserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/rs/zerolog"
)

const APIVersion = "2026-02-20"

type Client interface {
	CreateScan(ctx context.Context, goal string, strategies []string) (string, error)
	NextChats(ctx context.Context, scanID string, responses []ChatResponse) ([]ChatPrompt, error)
	GetStatus(ctx context.Context, scanID string) (*ScanStatus, error)
	GetResult(ctx context.Context, scanID string) (*ScanResult, error)
	ListGoals(ctx context.Context) ([]EnumEntry, error)
	ListStrategies(ctx context.Context) ([]EnumEntry, error)
}

type ClientImpl struct {
	baseURL    string
	tenantID   string
	httpClient *http.Client
	logger     *zerolog.Logger
}

var _ Client = (*ClientImpl)(nil)

func NewClient(logger *zerolog.Logger, httpClient *http.Client, baseURL, tenantID string) *ClientImpl {
	return &ClientImpl{
		baseURL:    baseURL,
		tenantID:   tenantID,
		httpClient: httpClient,
		logger:     logger,
	}
}

func (c *ClientImpl) CreateScan(ctx context.Context, goal string, strategies []string) (string, error) {
	body := CreateScanRequest{Goal: goal, Strategies: strategies}
	reqBytes, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal CreateScan request: %w", err)
	}

	url := fmt.Sprintf("%s/hidden/tenants/%s/red_team_scans?version=%s", c.baseURL, c.tenantID, APIVersion)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBytes))
	if err != nil {
		return "", fmt.Errorf("build CreateScan request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("CreateScan request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read CreateScan response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("CreateScan returned status %d: %s", resp.StatusCode, string(respBytes))
	}

	var result CreateScanResponse
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return "", fmt.Errorf("unmarshal CreateScan response: %w", err)
	}

	c.logger.Debug().Str("scanID", result.ScanID).Msg("scan created")
	return result.ScanID, nil
}

func (c *ClientImpl) NextChats(ctx context.Context, scanID string, responses []ChatResponse) ([]ChatPrompt, error) {
	if responses == nil {
		responses = []ChatResponse{}
	}
	body := NextChatsRequest{Chats: responses}
	reqBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal NextChats request: %w", err)
	}

	url := fmt.Sprintf("%s/hidden/tenants/%s/red_team_scans/%s/next?version=%s", c.baseURL, c.tenantID, scanID, APIVersion)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("build NextChats request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("NextChats request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read NextChats response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("NextChats returned status %d: %s", resp.StatusCode, string(respBytes))
	}

	var result NextChatsResponse
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return nil, fmt.Errorf("unmarshal NextChats response: %w", err)
	}

	return result.Chats, nil
}

func (c *ClientImpl) GetStatus(ctx context.Context, scanID string) (*ScanStatus, error) {
	url := fmt.Sprintf("%s/hidden/tenants/%s/red_team_scans/%s/status?version=%s", c.baseURL, c.tenantID, scanID, APIVersion)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("build GetStatus request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GetStatus request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read GetStatus response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GetStatus returned status %d: %s", resp.StatusCode, string(respBytes))
	}

	var result ScanStatus
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return nil, fmt.Errorf("unmarshal GetStatus response: %w", err)
	}

	return &result, nil
}

func (c *ClientImpl) GetResult(ctx context.Context, scanID string) (*ScanResult, error) {
	url := fmt.Sprintf("%s/hidden/tenants/%s/red_team_scans/%s?version=%s", c.baseURL, c.tenantID, scanID, APIVersion)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("build GetResult request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GetResult request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read GetResult response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("scan %s not found", scanID)
		}
		return nil, fmt.Errorf("GetResult returned status %d: %s", resp.StatusCode, string(respBytes))
	}

	var result ScanResult
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return nil, fmt.Errorf("unmarshal GetResult response: %w", err)
	}

	return &result, nil
}

func (c *ClientImpl) listEnum(ctx context.Context, endpoint string) ([]EnumEntry, error) {
	url := fmt.Sprintf("%s/hidden/%s?version=%s", c.baseURL, endpoint, APIVersion)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("build %s request: %w", endpoint, err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s request failed: %w", endpoint, err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read %s response: %w", endpoint, err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned status %d: %s", endpoint, resp.StatusCode, string(respBytes))
	}

	var entries []EnumEntry
	if err := json.Unmarshal(respBytes, &entries); err != nil {
		return nil, fmt.Errorf("unmarshal %s response: %w", endpoint, err)
	}

	return entries, nil
}

func (c *ClientImpl) ListGoals(ctx context.Context) ([]EnumEntry, error) {
	return c.listEnum(ctx, "goals")
}

func (c *ClientImpl) ListStrategies(ctx context.Context) ([]EnumEntry, error) {
	return c.listEnum(ctx, "strategies")
}
