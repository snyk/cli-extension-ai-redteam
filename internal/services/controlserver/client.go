package controlserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/rs/zerolog"

	redteam_errors "github.com/snyk/cli-extension-ai-redteam/internal/errors/redteam"
	"github.com/snyk/cli-extension-ai-redteam/internal/utils"
)

const APIVersion = "2026-02-20"

type Client interface {
	CreateScan(ctx context.Context, req *CreateScanRequest) (string, error)
	NextChats(ctx context.Context, scanID string, responses []ChatResponse) ([]ChatPrompt, error)
	GetStatus(ctx context.Context, scanID string) (*ScanStatus, error)
	GetResult(ctx context.Context, scanID string) (*ScanResult, error)
	GetReport(ctx context.Context, scanID string) (json.RawMessage, error)
	ListGoals(ctx context.Context) ([]EnumEntry, error)
	ListStrategies(ctx context.Context) ([]EnumEntry, error)
	ListProfiles(ctx context.Context) ([]ProfileResponse, error)
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

func httpError(op string, statusCode int, body []byte) error {
	detail := fmt.Sprintf("%s returned status %d: %s", op, statusCode, utils.TruncateBody(body))
	return redteam_errors.ErrorFromHTTPStatus(statusCode, detail)
}

func (c *ClientImpl) CreateScan(ctx context.Context, req *CreateScanRequest) (string, error) {
	if req == nil {
		req = &CreateScanRequest{}
	}
	body := *req
	reqBytes, err := json.Marshal(body)
	if err != nil {
		return "", redteam_errors.NewInternalError(fmt.Sprintf("marshal CreateScan request: %s", err))
	}

	urlStr := fmt.Sprintf("%s/hidden/tenants/%s/red_team_scans?version=%s", c.baseURL, c.tenantID, APIVersion)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, urlStr, bytes.NewReader(reqBytes))
	if err != nil {
		return "", redteam_errors.NewInternalError(fmt.Sprintf("build CreateScan request: %s", err))
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", utils.ErrorFromHTTPClient("CreateScan", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", redteam_errors.NewInternalError(fmt.Sprintf("read CreateScan response: %s", err))
	}

	if resp.StatusCode != http.StatusOK {
		return "", httpError("CreateScan", resp.StatusCode, respBytes)
	}

	var result CreateScanResponse
	if err := json.Unmarshal(respBytes, &result); err != nil {
		msg := fmt.Sprintf("unmarshal CreateScan response: %s", err)
		if len(respBytes) > 0 && respBytes[0] == '<' {
			msg += " (server returned an unexpected response; verify configuration and that the service is available)"
		}
		return "", redteam_errors.NewInternalError(msg)
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
		return nil, redteam_errors.NewInternalError(fmt.Sprintf("marshal NextChats request: %s", err))
	}

	url := fmt.Sprintf("%s/hidden/tenants/%s/red_team_scans/%s/next?version=%s", c.baseURL, c.tenantID, scanID, APIVersion)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBytes))
	if err != nil {
		return nil, redteam_errors.NewInternalError(fmt.Sprintf("build NextChats request: %s", err))
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, utils.ErrorFromHTTPClient("NextChats", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, redteam_errors.NewInternalError(fmt.Sprintf("read NextChats response: %s", err))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, httpError("NextChats", resp.StatusCode, respBytes)
	}

	var result NextChatsResponse
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return nil, redteam_errors.NewInternalError(fmt.Sprintf("unmarshal NextChats response: %s", err))
	}

	return result.Chats, nil
}

func (c *ClientImpl) doGet(ctx context.Context, op, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, redteam_errors.NewInternalError(fmt.Sprintf("build %s request: %s", op, err))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, utils.ErrorFromHTTPClient(op, err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, redteam_errors.NewInternalError(fmt.Sprintf("read %s response: %s", op, err))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, httpError(op, resp.StatusCode, respBytes)
	}

	return respBytes, nil
}

func (c *ClientImpl) GetStatus(ctx context.Context, scanID string) (*ScanStatus, error) {
	url := fmt.Sprintf(
		"%s/hidden/tenants/%s/red_team_scans/%s/status?version=%s",
		c.baseURL, c.tenantID, scanID, APIVersion,
	)
	respBytes, err := c.doGet(ctx, "GetStatus", url)
	if err != nil {
		return nil, err
	}

	var result ScanStatus
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return nil, redteam_errors.NewInternalError(
			fmt.Sprintf("unmarshal GetStatus response: %s", err),
		)
	}
	return &result, nil
}

func (c *ClientImpl) GetResult(ctx context.Context, scanID string) (*ScanResult, error) {
	url := fmt.Sprintf(
		"%s/hidden/tenants/%s/red_team_scans/%s?version=%s",
		c.baseURL, c.tenantID, scanID, APIVersion,
	)
	respBytes, err := c.doGet(ctx, "GetResult", url)
	if err != nil {
		return nil, err
	}

	var result ScanResult
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return nil, redteam_errors.NewInternalError(
			fmt.Sprintf("unmarshal GetResult response: %s", err),
		)
	}
	return &result, nil
}

func (c *ClientImpl) GetReport(ctx context.Context, scanID string) (json.RawMessage, error) {
	url := fmt.Sprintf("%s/hidden/tenants/%s/red_team_scans/%s/report?version=%s", c.baseURL, c.tenantID, scanID, APIVersion)
	respBytes, err := c.doGet(ctx, "GetReport", url)
	if err != nil {
		return nil, err	}

	return json.RawMessage(respBytes), nil
}

func (c *ClientImpl) listEnum(ctx context.Context, endpoint string) ([]EnumEntry, error) {
	url := fmt.Sprintf("%s/hidden/%s?version=%s", c.baseURL, endpoint, APIVersion)
	respBytes, err := c.doGet(ctx, endpoint, url)
	if err != nil {
		return nil, err
	}

	var entries []EnumEntry
	if err := json.Unmarshal(respBytes, &entries); err != nil {
		return nil, redteam_errors.NewInternalError(fmt.Sprintf("unmarshal %s response: %s", endpoint, err))
	}

	return entries, nil
}

func (c *ClientImpl) ListGoals(ctx context.Context) ([]EnumEntry, error) {
	return c.listEnum(ctx, "goals")
}

func (c *ClientImpl) ListStrategies(ctx context.Context) ([]EnumEntry, error) {
	return c.listEnum(ctx, "strategies")
}

func (c *ClientImpl) ListProfiles(ctx context.Context) ([]ProfileResponse, error) {
	url := fmt.Sprintf("%s/hidden/profiles?version=%s", c.baseURL, APIVersion)
	respBytes, err := c.doGet(ctx, "ListProfiles", url)
	if err != nil {
		return nil, err	}

	var profiles []ProfileResponse
	if err := json.Unmarshal(respBytes, &profiles); err != nil {
		return nil, redteam_errors.NewInternalError(fmt.Sprintf("unmarshal ListProfiles response: %s", err))
	}

	return profiles, nil
}
