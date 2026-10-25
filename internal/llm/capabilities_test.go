package llm

import (
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
)

func TestProviderCapabilitiesDescribeImplementedProtocols(t *testing.T) {
	for _, provider := range []config.ModelProvider{config.OpenAIProvider, config.GeminiProvider, config.VertexProvider, config.PiProvider, config.AgyProvider} {
		profile := config.ModelProfile{Provider: provider}
		capabilities := ProviderCapabilities(profile)
		if !capabilities.Completion || !capabilities.JSONSchema || capabilities.Tools == profile.IsCLI() || capabilities.Streaming == profile.IsCLI() || capabilities.Attachments || capabilities.Grounding || capabilities.LiveSessions {
			t.Fatalf("%s capabilities=%+v", provider, capabilities)
		}
	}
	if ProviderCapabilities(config.ModelProfile{Provider: "unknown"}) != (Capabilities{}) {
		t.Fatal("unknown adapter claimed capabilities")
	}
}

func TestGoogleSearchCapabilitiesRequireKnownNativeTextProfiles(t *testing.T) {
	for _, provider := range []config.ModelProvider{config.GeminiProvider, config.VertexProvider} {
		if !ProviderCapabilities(config.ModelProfile{Provider: provider, Model: "gemini-2.5-flash"}).Grounding {
			t.Fatal("native search protocol unavailable")
		}
	}
	for _, profile := range []config.ModelProfile{{Provider: config.OpenAIProvider, Model: "gemini-2.5-flash"}, {Provider: config.GeminiProvider, Model: "unknown"}, {Provider: config.GeminiProvider, Model: "gemini-image"}} {
		if ProviderCapabilities(profile).Grounding {
			t.Fatal("unsupported profile advertised search")
		}
	}
}

func TestAttachmentCapabilitiesRequireKnownNativeMultimodalProfiles(t *testing.T) {
	for _, provider := range []config.ModelProvider{config.GeminiProvider, config.VertexProvider} {
		if !ProviderCapabilities(config.ModelProfile{Provider: provider, Model: "gemini-3.8-flash"}).Attachments {
			t.Fatal("native attachment capability unavailable")
		}
	}
	for _, profile := range []config.ModelProfile{{Provider: config.OpenAIProvider, Model: "gemini-3.8-flash"}, {Provider: config.GeminiProvider, Model: "gemini-3.8-live"}, {Provider: config.GeminiProvider, Model: "unknown"}, {Provider: config.PiProvider, Model: "gemini-2.5-flash"}} {
		if ProviderCapabilities(profile).Attachments {
			t.Fatal("unsupported attachment protocol advertised")
		}
	}
}
