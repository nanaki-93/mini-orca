package config

import (
	"fmt"
	"regexp"
	"slices"
)

// CloudTarget contains references only. Reading configuration never discovers
// credentials or connects to a cloud service.
type CloudTarget struct {
	ID                string `yaml:"id" json:"id"`
	Service           string `yaml:"service" json:"service"`
	Project           string `yaml:"project" json:"project"`
	Region            string `yaml:"region" json:"region"`
	Resource          string `yaml:"resource" json:"resource"`
	Principal         string `yaml:"principal" json:"principal"`
	CredentialFileEnv string `yaml:"credential_file_env" json:"credential_file_env"`
	RetentionDays     int    `yaml:"retention_days" json:"retention_days"`
}

var cloudProject = regexp.MustCompile(`^[1-9][0-9]{5,19}$`)
var cloudRegion = regexp.MustCompile(`^[a-z]{2,16}-[a-z]{2,16}[0-9]{1,2}$`)
var cloudEngine = regexp.MustCompile(`^[1-9][0-9]{0,30}$`)
var cloudBucket = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$`)
var cloudPrincipal = regexp.MustCompile(`^[a-z][a-z0-9-]{4,28}[a-z0-9]@[a-z][a-z0-9-]{4,28}[a-z0-9]\.iam\.gserviceaccount\.com$`)

func (c CloudTarget) Validate() error {
	if !integrationID.MatchString(c.ID) || !cloudProject.MatchString(c.Project) || !cloudRegion.MatchString(c.Region) {
		return fmt.Errorf("cloud target requires an ID, Google project number and regional location")
	}
	if !cloudPrincipal.MatchString(c.Principal) || !integrationEnvName.MatchString(c.CredentialFileEnv) || c.RetentionDays < 1 || c.RetentionDays > 365 {
		return fmt.Errorf("cloud target requires an explicit service-account principal, credential-file environment reference and 1–365 day cleanup deadline")
	}
	switch c.Service {
	case "vertex_session":
		if !cloudEngine.MatchString(c.Resource) {
			return fmt.Errorf("vertex resource must be a reasoning-engine ID")
		}
	case "gcs_artifact":
		if !cloudBucket.MatchString(c.Resource) {
			return fmt.Errorf("GCS resource must be a bucket name without a path")
		}
	default:
		return fmt.Errorf("cloud service must be vertex_session or gcs_artifact; the pinned Memory Bank adapter cannot satisfy identity and deletion controls")
	}
	return nil
}

func resolveCloudTargets(input []CloudTarget, seen map[string]bool) ([]CloudTarget, error) {
	if len(input) > 4 {
		return nil, fmt.Errorf("integrations.cloud permits at most four destinations")
	}
	result := slices.Clone(input)
	for _, c := range result {
		if err := c.Validate(); err != nil {
			return nil, fmt.Errorf("cloud destination: %w", err)
		}
		if seen[c.ID] {
			return nil, fmt.Errorf("cloud destination IDs must be unique across integrations")
		}
		seen[c.ID] = true
	}
	return result, nil
}
