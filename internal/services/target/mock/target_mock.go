package targetmock

import (
	"context"

	"github.com/snyk/cli-extension-ai-redteam/internal/services/target"
)

type MockClient struct {
	Responses map[string]string
	Error     error
	Calls     []string
}

var _ target.Client = (*MockClient)(nil)

func (m *MockClient) SendPrompt(_ context.Context, prompt string) (string, error) {
	m.Calls = append(m.Calls, prompt)
	if m.Error != nil {
		return "", m.Error
	}
	if resp, ok := m.Responses[prompt]; ok {
		return resp, nil
	}
	return "mock response", nil
}
