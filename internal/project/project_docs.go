package project

import "github.com/nanaki-93/mini-orca/v2/internal/project/handbook"

type ProjectDocsContext = handbook.ProjectDocsContext
type ProjectDocsDependency = handbook.ProjectDocsDependency
type ProjectDocsContract = handbook.ProjectDocsContract
type ProjectDocsComponent = handbook.ProjectDocsComponent
type ProjectDocsScenarioStep = handbook.ProjectDocsScenarioStep
type ProjectDocsScenario = handbook.ProjectDocsScenario
type ProjectDocsDecision = handbook.ProjectDocsDecision
type ProjectDocsRunbookItem = handbook.ProjectDocsRunbookItem
type ProjectDocsGlossaryItem = handbook.ProjectDocsGlossaryItem
type ProjectDocsReport = handbook.ProjectDocsReport

// BuildProjectDocs synthesizes a language-agnostic engineering specification
// from the project's analysis report and deterministic file index.
func BuildProjectDocs(analysis Analysis, index *ProjectIndex) ProjectDocsReport {
	return handbook.BuildProjectDocs(analysis, index)
}

// StoreProjectDocsReport atomically persists the engineering specification under .mini-orca.
func StoreProjectDocsReport(root string, report ProjectDocsReport) error {
	return handbook.StoreProjectDocsReport(root, report)
}

// LoadProjectDocsReport reads a stored engineering specification and marks it
// stale when the project revision has changed.
func LoadProjectDocsReport(root, projectID, projectRevision string) (*ProjectDocsReport, error) {
	return handbook.LoadProjectDocsReport(root, projectID, projectRevision)
}
