package controlservermock

import (
	"context"

	"github.com/snyk/cli-extension-ai-redteam/internal/services/controlserver"
)

type MockClient struct {
	ScanID        string
	ChatRound     int
	ChatSeqs      [][]controlserver.ChatPrompt
	Status        *controlserver.ScanStatus
	Result        *controlserver.ScanResult
	Goals         []controlserver.EnumEntry
	Strategies    []controlserver.EnumEntry
	CreateErr     error
	NextErr       error
	StatusErr     error
	ResultErr     error
	GoalsErr      error
	StrategiesErr error

	// Capture CreateScan arguments for tests (set when CreateScan is called).
	CreateScanRequest *controlserver.CreateScanRequest
}

var _ controlserver.Client = (*MockClient)(nil)

func (m *MockClient) CreateScan(_ context.Context, req *controlserver.CreateScanRequest) (string, error) {
	m.CreateScanRequest = req
	if m.CreateErr != nil {
		return "", m.CreateErr
	}
	return m.ScanID, nil
}

func (m *MockClient) NextChats(_ context.Context, _ string, _ []controlserver.ChatResponse) ([]controlserver.ChatPrompt, error) {
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
