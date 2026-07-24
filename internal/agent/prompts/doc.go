// Package prompts provides prompt templates for each agent.
//
// Prompts are Go templates that combine:
//   1. A system message defining the agent's role
//   2. The current session context (project structure, previous work)
//   3. The agent's skills (injected dynamically)
//   4. Phase-specific instructions
//   5. Output format requirements
//
// # Template Structure
//
// Each prompt is a Go template with the following variables:
//   - .ProjectStructure: The project's file tree
//   - .CurrentTask: The task the agent should work on
//   - .PreviousWork: Context from previous agent executions
//   - .Skills: The agent's configured skills (from agent.GetSkillPrompt)
//   - .Rules: Phase-specific rules and constraints
//
// # Customization
//
// Prompts can be customized per-project by providing a custom template
// directory. The default templates are optimized for Go projects but
// work with any language supported by Mini-Orca.
//
// # Example
//
//	prompt := prompts.Planner(ctx, session, plan, skills)
package prompts
