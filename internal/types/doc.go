// Package types provides shared type definitions for Mini-Orca.
//
// This package contains types that are used across multiple packages
// to avoid import cycles and maintain consistency.
//
// # Types
//
//   - Skill: A capability that an agent can use (knowledge or tool)
//   - SkillType: Enum for skill categories
//
// # Example
//
//	skill := types.Skill{
//	    Name:        "solid_principles",
//	    Description: "Apply SOLID design principles",
//	    Type:        types.SkillTypeKnowledge,
//	    Priority:    1,
//	    Prompt:      "Follow single responsibility, open-closed, etc.",
//	}
package types
