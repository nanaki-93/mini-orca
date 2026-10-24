package llm

import "github.com/nanaki-93/mini-orca/v2/internal/config"

// Capabilities describes implemented adapter protocols, not an account's model
// entitlements. A particular model can still reject a supported request format.
type Capabilities struct {
	Completion   bool `json:"completion"`
	JSONSchema   bool `json:"json_schema"`
	Tools        bool `json:"tools"`
	Streaming    bool `json:"streaming"`
	Attachments  bool `json:"attachments"`
	Grounding    bool `json:"grounding"`
	LiveSessions bool `json:"live_sessions"`
}

func ProviderCapabilities(profile config.ModelProfile) Capabilities {
	if profile.IsCLI() {
		return Capabilities{Completion: true, JSONSchema: true}
	}
	if profile.Provider == "" || profile.Provider == config.OpenAIProvider || profile.IsGoogle() {
		return Capabilities{Completion: true, JSONSchema: true, Tools: true, Streaming: true, Grounding: SupportsGoogleSearch(profile)}
	}
	return Capabilities{}
}

// Search support is deliberately narrower than generic native completion.
// Account entitlement and provider availability remain runtime responses.
func SupportsGoogleSearch(profile config.ModelProfile) bool {
	if !profile.IsGoogle() {
		return false
	}
	switch profile.Model {
	case "gemini-2.5-pro", "gemini-2.5-flash", "gemini-2.5-flash-lite", "gemini-3.8-flash", "gemini-3.6-flash", "gemini-3.5-flash-lite", "gemini-3.1-pro-preview", "gemini-3-flash-preview":
		return true
	default:
		return false
	}
}
