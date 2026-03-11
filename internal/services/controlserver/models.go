package controlserver

type CreateScanRequest struct {
	Goal         string    `json:"goal,omitempty"`
	Strategies   []string  `json:"strategies,omitempty"`
	Purpose      *string   `json:"purpose,omitempty"`
	SystemPrompt *string   `json:"system_prompt,omitempty"`
	Tools        *[]string `json:"tools,omitempty"`
}

// GroundTruth is optional context sent when creating a scan. Only non-nil fields are included in the request.
type GroundTruth struct {
	Purpose      *string
	SystemPrompt *string
	Tools        *[]string
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
	Goal       string         `json:"goal"`
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
	Goal    string         `json:"goal"`
	Done    bool           `json:"done"`
	Attacks []AttackResult `json:"attacks"`
	Tags    []string       `json:"tags"`
}

type EnumEntry struct {
	Value        string `json:"value"`
	Description  string `json:"description"`
	DisplayOrder int    `json:"display_order"`
}
