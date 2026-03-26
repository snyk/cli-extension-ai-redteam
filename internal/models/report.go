package models

// ScanReport mirrors the backend's ScanReport response from
// GET /red_team_scans/{id}/report.
type ScanReport struct {
	ID          string             `json:"id"`
	Results     []ReportFinding    `json:"results"`
	PassedTypes []ReportPassedType `json:"passed_types,omitempty"`
}

type ReportPassedType struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ReportFinding struct {
	ID         string                  `json:"id"`
	Definition ReportFindingDefinition `json:"definition"`
	Tags       []string                `json:"tags,omitempty"`
	Severity   string                  `json:"severity"`
	URL        string                  `json:"url"`
	Turns      []ReportFindingTurn     `json:"turns,omitempty"`
	Evidence   ReportFindingEvidence   `json:"evidence,omitempty"`
}

type ReportFindingTurn struct {
	Request  *string `json:"request,omitempty"`
	Response *string `json:"response,omitempty"`
}

type ReportFindingEvidence struct {
	Type    string                `json:"type"`
	Content ReportEvidenceContent `json:"content"`
}

type ReportEvidenceContent struct {
	Reason string `json:"reason"`
}

type ReportFindingDefinition struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}
