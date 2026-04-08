package controlservermock

import (
	"context"
	"encoding/json"

	"github.com/snyk/cli-extension-ai-redteam/internal/services/controlserver"
)

type MockClient struct {
	ScanID        string
	ChatRound     int
	ChatSeqs      [][]controlserver.ChatPrompt
	Status        *controlserver.ScanStatus
	Result        *controlserver.ScanResult
	Report        json.RawMessage
	Goals         []controlserver.EnumEntry
	Strategies    []controlserver.EnumEntry
	Profiles      []controlserver.ProfileResponse
	Targets       []controlserver.TargetListItem
	Target        *controlserver.TargetResponse
	CreatedTarget *controlserver.TargetResponse
	CreateErr     error
	NextErr       error
	StatusErr     error
	ResultErr     error
	ReportErr     error
	GoalsErr      error
	StrategiesErr error
	ProfilesErr   error
	TargetsErr    error
	TargetErr     error
	CreateTgtErr  error
	UpdatedTarget *controlserver.TargetResponse
	UpdateTgtErr  error
	DeleteTgtErr  error

	// Capture CreateScan arguments for tests (set when CreateScan is called).
	CreateScanRequest *controlserver.CreateScanRequest
	// Capture CreateTarget arguments for tests.
	CreateTargetRequest *controlserver.TargetCreateRequest
	// Capture UpdateTarget arguments for tests.
	UpdateTargetName    string
	UpdateTargetRequest *controlserver.TargetUpdateRequest
	// Capture DeleteTarget target name.
	DeletedTargetName string
}

var _ controlserver.Client = (*MockClient)(nil)

func (m *MockClient) CreateScan(_ context.Context, req *controlserver.CreateScanRequest) (string, error) {
	m.CreateScanRequest = req
	if m.CreateErr != nil {
		return "", m.CreateErr
	}
	return m.ScanID, nil
}

func (m *MockClient) NextChats(
	_ context.Context, _ string, _ []controlserver.ChatResponse,
) ([]controlserver.ChatPrompt, error) {
	if m.NextErr != nil {
		return nil, m.NextErr
	}
	if m.ChatRound < len(m.ChatSeqs) {
		chats := m.ChatSeqs[m.ChatRound]
		m.ChatRound++
		return chats, nil
	}
	return []controlserver.ChatPrompt{}, nil
}

func (m *MockClient) GetStatus(_ context.Context, _ string) (*controlserver.ScanStatus, error) {
	if m.StatusErr != nil {
		return nil, m.StatusErr
	}
	return m.Status, nil
}

func (m *MockClient) GetResult(_ context.Context, _ string) (*controlserver.ScanResult, error) {
	if m.ResultErr != nil {
		return nil, m.ResultErr
	}
	return m.Result, nil
}

func (m *MockClient) GetReport(_ context.Context, _ string) (json.RawMessage, error) {
	if m.ReportErr != nil {
		return nil, m.ReportErr
	}
	return m.Report, nil
}

func (m *MockClient) ListGoals(_ context.Context) ([]controlserver.EnumEntry, error) {
	if m.GoalsErr != nil {
		return nil, m.GoalsErr
	}
	return m.Goals, nil
}

func (m *MockClient) ListStrategies(_ context.Context) ([]controlserver.EnumEntry, error) {
	if m.StrategiesErr != nil {
		return nil, m.StrategiesErr
	}
	return m.Strategies, nil
}

func (m *MockClient) ListProfiles(_ context.Context) ([]controlserver.ProfileResponse, error) {
	if m.ProfilesErr != nil {
		return nil, m.ProfilesErr
	}
	return m.Profiles, nil
}

func (m *MockClient) ListTargets(_ context.Context) ([]controlserver.TargetListItem, error) {
	if m.TargetsErr != nil {
		return nil, m.TargetsErr
	}
	return m.Targets, nil
}

func (m *MockClient) GetTarget(_ context.Context, _ string) (*controlserver.TargetResponse, error) {
	if m.TargetErr != nil {
		return nil, m.TargetErr
	}
	return m.Target, nil
}

func (m *MockClient) CreateTarget(_ context.Context, req *controlserver.TargetCreateRequest) (*controlserver.TargetResponse, error) {
	m.CreateTargetRequest = req
	if m.CreateTgtErr != nil {
		return nil, m.CreateTgtErr
	}
	return m.CreatedTarget, nil
}

func (m *MockClient) UpdateTarget(_ context.Context, targetName string, req *controlserver.TargetUpdateRequest) (*controlserver.TargetResponse, error) {
	m.UpdateTargetName = targetName
	m.UpdateTargetRequest = req
	if m.UpdateTgtErr != nil {
		return nil, m.UpdateTgtErr
	}
	return m.UpdatedTarget, nil
}

func (m *MockClient) DeleteTarget(_ context.Context, targetName string) error {
	m.DeletedTargetName = targetName
	if m.DeleteTgtErr != nil {
		return m.DeleteTgtErr
	}
	return nil
}
