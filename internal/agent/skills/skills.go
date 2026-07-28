package skills

// SkillType represents the category of a skill.
type SkillType string

const (
	// Knowledge skills provide domain expertise and guidance.
	Knowledge SkillType = "knowledge"
	// Tool skills provide executable capabilities.
	Tool SkillType = "tool"
)

// SkillCategory represents the functional category of a skill.
type SkillCategory string

const (
	// Design category covers architecture and design skills.
	Design SkillCategory = "design"
	// Coding category covers implementation skills.
	Coding SkillCategory = "coding"
	// Testing category covers verification skills.
	Testing SkillCategory = "testing"
	// Review category covers quality assurance skills.
	Review SkillCategory = "review"
	// Principles category covers best practice and methodology skills.
	Principles SkillCategory = "principles"
)

// Skill represents a reusable capability that an agent can use.
type Skill struct {
	// Name is the unique identifier for the skill.
	Name string

	// Type categorizes the skill as knowledge or tool.
	Type SkillType

	// Category groups the skill by functional area.
	Category SkillCategory

	// Description provides a human-readable explanation of the skill.
	Description string

	// PromptTemplate is the template used to generate the skill prompt.
	PromptTemplate string

	// Parameters holds additional configuration for the skill.
	Parameters map[string]string
}
