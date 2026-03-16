package targetmock

import (
	"context"

	"github.com/snyk/cli-extension-ai-redteam/internal/services/target"
)

type MockClient struct {
	Responses                 map[string]string
	Error                     error
	Calls                     []string
	FailuresBeforeCircuitOpen int
	failureCount              int
	PingResponse              *target.PingResult
}

var _ target.Client = (*MockClient)(nil)

func (m *MockClient) Ping(_ context.Context) target.PingResult {
	if m.PingResponse != nil {
		return *m.PingResponse
	}
	return target.PingResult{
		Success:    true,
		Suggestion: "Target is reachable and responding correctly.",
	}
}

func (m *MockClient) SendPrompt(_ context.Context, prompt string) (string, error) {
	m.Calls = append(m.Calls, prompt)
	if m.Error != nil {
		m.failureCount++
		if m.FailuresBeforeCircuitOpen > 0 && m.failureCount > m.FailuresBeforeCircuitOpen {
			return "", target.ErrCircuitOpen
		}
		return "", m.Error
	}
	if resp, ok := m.Responses[prompt]; ok {
		return resp, nil
	}
	return "mock response", nil
}
