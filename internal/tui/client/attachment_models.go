package client

type AttachmentImport struct {
	ProjectIdentity
	Name          string `json:"name"`
	MIME          string `json:"mime"`
	Data          []byte `json:"data"`
	ConfirmImport bool   `json:"confirm_import"`
}

type AttachmentReference struct {
	ID      string `json:"id"`
	Version int64  `json:"version"`
	Hash    string `json:"hash"`
}

type AttachmentOverview struct {
	ProjectIdentity
	State           string          `json:"state"`
	Configuration   string          `json:"configuration"`
	SettingsVersion int64           `json:"settings_version"`
	Model           Model           `json:"model"`
	Limits          AgentLimits     `json:"limits"`
	Attachments     []AgentArtifact `json:"attachments"`
	Truncated       bool            `json:"truncated"`
}

type AttachmentAdviceRequest struct {
	ProjectIdentity
	Configuration         string                `json:"configuration"`
	SettingsVersion       int64                 `json:"settings_version"`
	Model                 string                `json:"model"`
	ProviderOrigin        string                `json:"provider_origin"`
	Question              string                `json:"question"`
	Attachments           []AttachmentReference `json:"attachments"`
	ConfirmRemoteProvider bool                  `json:"confirm_remote_provider"`
	ConfirmSourceSharing  bool                  `json:"confirm_source_sharing"`
}

type AttachmentUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	PromptDetails    *struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"prompt_tokens_details,omitempty"`
}

type AttachmentAdviceReceipt struct {
	State      string          `json:"state"`
	Artifact   AgentArtifact   `json:"artifact"`
	Model      string          `json:"model"`
	Usage      AttachmentUsage `json:"usage"`
	UsageKnown bool            `json:"usage_known"`
}
