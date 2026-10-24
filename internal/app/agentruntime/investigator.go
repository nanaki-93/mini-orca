package agentruntime

import (
	"fmt"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/genai"

	"github.com/nanaki-93/mini-orca/v2/internal/llm"
)

func newInvestigationAgent(client CompletionClient, request Request) (agent.Agent, error) {
	if len(request.Messages) != 1 || request.Messages[0].Role != "user" || request.Stream {
		return nil, fmt.Errorf("%w: investigation requires one admitted user prompt", llm.ErrRequestRejected)
	}
	adapter, err := investigationAdapter(client, request.Schema)
	if err != nil {
		return nil, err
	}
	var tools []tool.Tool
	for _, item := range request.Investigation.tools {
		tools = append(tools, item)
	}
	config := llmagent.Config{
		Name: "investigation", Description: "Investigate the explicitly admitted repository scope",
		Model: &investigationModel{inner: adapter, scope: request.Investigation, contextTokens: request.ContextMaxTokens, protected: request.Messages[0].Content}, Tools: tools,
		DisallowTransferToParent: true, DisallowTransferToPeers: true,
	}
	if request.Schema != nil {
		// OutputSchema installs an SDK tool. The transport's response schema
		// constrains the final answer without adding executable authority.
		config.GenerateContentConfig = &genai.GenerateContentConfig{ResponseMIMEType: "application/json", ResponseJsonSchema: request.Schema.Schema}
	}
	worker, err := llmagent.New(config)
	if err != nil {
		return nil, err
	}
	return agent.New(agent.Config{Name: "investigation_runner", Description: "Bounded repository investigation", Run: investigationRun(worker, request.Investigation)})
}

func investigationAdapter(client CompletionClient, schema *llm.JSONSchema) (model.LLM, error) {
	if provider, ok := client.(*ProviderClient); ok && provider.profile.IsGoogle() {
		return &googleModel{client: provider}, nil
	}
	transport, ok := client.(generationClient)
	if !ok {
		return nil, fmt.Errorf("%w: selected provider cannot use investigation tools", llm.ErrRequestRejected)
	}
	adapter := &compatibleModel{client: transport, schemaName: "investigation"}
	if schema != nil {
		adapter.schemaName = schema.Name
	}
	return adapter, nil
}
