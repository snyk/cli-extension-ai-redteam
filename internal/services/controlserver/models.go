package controlserver

type AttackEntry struct {
	Goal     string `json:"goal"`
	Strategy string `json:"strategy,omitempty"`
}

type ProfileResponse struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Entries     []AttackEntry `json:"entries"`
}

type CreateScanRequest struct {
	Attacks     []AttackEntry `json:"attacks"`
	Purpose     string        `json:"purpose,omitempty"`
	GroundTruth *GroundTruth  `json:"ground_truth,omitempty"`
	TargetURL   string        `json:"target_url,omitempty"`
	Mode        string        `json:"mode,omitempty"`
	TargetName  string        `json:"target_name,omitempty"`
}

// GroundTruth is optional context for judging: system prompt and tools of the target.
// Tools is a single string (e.g. comma-separated tool names).
type GroundTruth struct {
	SystemPrompt string `json:"system_prompt,omitempty"`
	Tools        string `json:"tools,omitempty"`
}

type CreateScanResponse struct {
	ScanID string `json:"scan_id"`
}

type ChatResponse struct {
	Seq      int    `json:"seq"`
	Response string `json:"response"`
}

type NextChatsRequest struct {
	Chats []ChatResponse `json:"chats"`
}

type ChatPrompt struct {
	Seq    int    `json:"seq"`
	Prompt string `json:"prompt"`
	ChatID string `json:"chat_id"`
}

type NextChatsResponse struct {
	Chats []ChatPrompt `json:"chats"`
}

type AttackStatus struct {
	AttackType string   `json:"attack_type"`
	TotalChats int      `json:"total_chats"`
	Completed  int      `json:"completed"`
	Successful int      `json:"successful"`
	Failed     int      `json:"failed"`
	Pending    int      `json:"pending"`
	Tags       []string `json:"tags"`
}

type ScanStatus struct {
	ScanID     string         `json:"scan_id"`
	Goals      []string       `json:"goals"`
	Done       bool           `json:"done"`
	TotalChats int            `json:"total_chats"`
	Completed  int            `json:"completed"`
	Successful int            `json:"successful"`
	Failed     int            `json:"failed"`
	Pending    int            `json:"pending"`
	Attacks    []AttackStatus `json:"attacks"`
	Tags       []string       `json:"tags"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatResult struct {
	Done     bool          `json:"done"`
	Success  bool          `json:"success"`
	Messages []ChatMessage `json:"messages"`
}

type AttackResult struct {
	AttackType string       `json:"attack_type"`
	Position   int          `json:"position"`
	Chats      []ChatResult `json:"chats"`
	Tags       []string     `json:"tags"`
}

type ScanResult struct {
	ScanID  string         `json:"scan_id"`
	Goals   []string       `json:"goals"`
	Done    bool           `json:"done"`
	Attacks []AttackResult `json:"attacks"`
	Tags    []string       `json:"tags"`
}

type EnumEntry struct {
	Value        string   `json:"value"`
	Description  string   `json:"description"`
	DisplayOrder int      `json:"display_order"`
	Strategies   []string `json:"strategies,omitempty"`
}

type TargetCreateRequest struct {
	Name   string         `json:"name"`
	Config map[string]any `json:"config"`
}

type TargetUpdateRequest struct {
	Name   string         `json:"name,omitempty"`
	Config map[string]any `json:"config,omitempty"`
}

type TargetResponse struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Config    map[string]any `json:"config"`
	CreatedAt string         `json:"created_at"`
	UpdatedAt string         `json:"updated_at"`
}

type TargetListItem struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}
