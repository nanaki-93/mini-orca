package types

// SkillType represents the type of skill.
type SkillType string

const (
	SkillTypeKnowledge SkillType = "knowledge"
	SkillTypeTool      SkillType = "tool"
)

// Skill represents a capability that an agent can use.
type Skill struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Type        SkillType  `json:"type"`
	Priority    int        `json:"priority"`
	Prompt      string     `json:"prompt"`
}
