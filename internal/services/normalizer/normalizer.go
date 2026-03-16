package normalizer

import (
	"fmt"
	"strings"

	"github.com/snyk/cli-extension-ai-redteam/internal/models"
	"github.com/snyk/cli-extension-ai-redteam/internal/services/controlserver"
)

func Normalize(result *controlserver.ScanResult, status *controlserver.ScanStatus, targetURL string) *models.GetAIVulnerabilitiesResponseData {
	var vulns []models.AIVulnerability
	for _, attack := range result.Attacks {
		for i, chat := range attack.Chats {
			if !chat.Success {
				continue
			}
			vulnID := fmt.Sprintf("%s-%s-chat-%d", result.ScanID, slugFromAttackType(attack.AttackType), i)
			vulns = append(vulns, models.AIVulnerability{
				ID:         vulnID,
				Definition: definitionFromAttack(attack.AttackType),
				Tags:       attack.Tags,
				Severity:   "high", // TODO(pkey): use /report endpoint for per-goal severity
				URL:        targetURL,
				Turns:      turnsFromMessages(chat.Messages),
				Evidence:   evidenceFromChat(chat),
			})
		}
	}

	if vulns == nil {
		vulns = []models.AIVulnerability{}
	}

	data := &models.GetAIVulnerabilitiesResponseData{
		ID:      result.ScanID,
		Results: vulns,
	}

	if status != nil {
		data.Summary = buildSummary(status)
	}

	return data
}

func buildSummary(status *controlserver.ScanStatus) *models.AIScanSummary {
	vulnSummaries := make([]models.AIScanSummaryVulnerability, 0, len(status.Attacks))
	for _, attack := range status.Attacks {
		slug := slugFromAttackType(attack.AttackType)
		name := nameFromAttackType(attack.AttackType)
		statusStr := "completed"
		if attack.Pending > 0 {
			statusStr = "in_progress"
		}
		vulnSummaries = append(vulnSummaries, models.AIScanSummaryVulnerability{
			EngineTag:   attack.AttackType,
			Slug:        slug,
			Name:        name,
			Description: fmt.Sprintf("Attack: %s", name),
			Severity:    "high", // TODO(pkey): use /report endpoint for per-goal severity
			Status:      statusStr,
			Vulnerable:  attack.Successful > 0,
			TotalChats:  attack.TotalChats,
			Successful:  attack.Successful,
			Failed:      attack.TotalChats - attack.Successful,
		})
	}
	return &models.AIScanSummary{Vulnerabilities: vulnSummaries}
}

func turnsFromMessages(messages []controlserver.ChatMessage) []models.Turn {
	var turns []models.Turn
	var currentRequest *string

	for _, msg := range messages {
		content := msg.Content
		switch msg.Role {
		case "minired":
			currentRequest = &content
		case "target":
			turn := models.Turn{
				Request:  currentRequest,
				Response: &content,
			}
			turns = append(turns, turn)
			currentRequest = nil
		}
	}

	if currentRequest != nil {
		turns = append(turns, models.Turn{Request: currentRequest})
	}

	return turns
}

func evidenceFromChat(chat controlserver.ChatResult) models.AIVulnerabilityEvidence {
	var lastTarget string
	for _, msg := range chat.Messages {
		if msg.Role == "target" {
			lastTarget = msg.Content
		}
	}
	return models.AIVulnerabilityEvidence{
		Type: "raw",
		Content: models.AIVulnerabilityEvidenceContent{
			Reason: lastTarget,
		},
	}
}

func slugFromAttackType(attackType string) string {
	parts := strings.Split(attackType, "/")
	if len(parts) >= 2 {
		return strings.Join(parts[:len(parts)-1], "/")
	}
	return attackType
}

func nameFromAttackType(attackType string) string {
	parts := strings.Split(attackType, "/")
	if len(parts) >= 2 {
		return strings.ReplaceAll(strings.ReplaceAll(parts[1], "_", " "), "-", " ")
	}
	return strings.ReplaceAll(strings.ReplaceAll(attackType, "_", " "), "-", " ")
}

func definitionFromAttack(attackType string) models.AIVulnerabilityDefinition {
	slug := slugFromAttackType(attackType)
	name := nameFromAttackType(attackType)
	return models.AIVulnerabilityDefinition{
		ID:          slug,
		Name:        name,
		Description: fmt.Sprintf("Successful attack via %s", name),
	}
}
