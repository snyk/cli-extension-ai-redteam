package models

type GetAIVulnerabilitiesResponseData struct {
	ID          string            `json:"id"`
	Results     []AIVulnerability `json:"results"`
	Summary     *AIScanSummary    `json:"summary,omitempty"`
	PassedTypes []PassedType      `json:"passed_types,omitempty"`
}

type PassedType struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type AIVulnerability struct {
	ID         string                    `json:"id"`
	Definition AIVulnerabilityDefinition `json:"definition"`
	Tags       []string                  `json:"tags,omitempty"`
	Severity   string                    `json:"severity"`
	URL        string                    `json:"url"`
	Turns      []Turn                    `json:"turns,omitempty"`
	Evidence   AIVulnerabilityEvidence   `json:"evidence,omitempty"`
}

type Turn struct {
	Request  *string `json:"request,omitempty"`
	Response *string `json:"response,omitempty"`
}

type AIVulnerabilityEvidence struct {
	Type    string                         `json:"type"`
	Content AIVulnerabilityEvidenceContent `json:"content"`
}

type AIVulnerabilityEvidenceContent struct {
	Reason string `json:"reason"`
}

type AIVulnerabilityDefinition struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type AIScanSummary struct {
	Vulnerabilities []AIScanSummaryVulnerability `json:"vulnerabilities"`
}

type AIScanSummaryVulnerability struct {
	EngineTag   string `json:"engine_tag"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
	Status      string `json:"status"`
	Vulnerable  bool   `json:"vulnerable"`
	TotalChats  int    `json:"total_chats"`
	Successful  int    `json:"successful"`
	Failed      int    `json:"failed"`
}
