package skills

// BuildSkillPrompt combines the prompt templates of the given skill names into a single prompt.
// It skips unknown skills and returns an empty string if no valid skills are found.
func (r *SkillsRegistry) BuildSkillPrompt(skillNames []string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var prompts []string
	for _, name := range skillNames {
		skill, exists := r.skills[name]
		if !exists {
			continue
		}
		if skill.PromptTemplate != "" {
			prompts = append(prompts, skill.PromptTemplate)
		}
	}

	if len(prompts) == 0 {
		return ""
	}

	result := ""
	for i, p := range prompts {
		if i > 0 {
			result += "\n\n---\n\n"
		}
		result += p
	}
	return result
}

// ValidateSkillNames checks each name against the registry and returns two slices:
// the first contains valid skill names, the second contains invalid ones.
func (r *SkillsRegistry) ValidateSkillNames(names []string) (valid, invalid []string) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	valid = make([]string, 0, len(names))
	invalid = make([]string, 0, len(names))

	for _, name := range names {
		if _, exists := r.skills[name]; exists {
			valid = append(valid, name)
		} else {
			invalid = append(invalid, name)
		}
	}

	return valid, invalid
}

// MergeSkills combines two skill name slices and deduplicates them,
// preserving the order of first occurrence.
func MergeSkills(agentSkills, globalSkills []string) []string {
	seen := make(map[string]struct{}, len(agentSkills)+len(globalSkills))
	result := make([]string, 0, len(agentSkills)+len(globalSkills))

	for _, name := range agentSkills {
		if _, ok := seen[name]; !ok {
			seen[name] = struct{}{}
			result = append(result, name)
		}
	}

	for _, name := range globalSkills {
		if _, ok := seen[name]; !ok {
			seen[name] = struct{}{}
			result = append(result, name)
		}
	}

	return result
}
