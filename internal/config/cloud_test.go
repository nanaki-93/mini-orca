package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestCloudConfigurationLoadsThroughStrictYAML(t *testing.T) {
	cfg := Default()
	cfg.Integrations.Cloud = []CloudTarget{{ID: "archive", Service: "gcs_artifact", Project: "123456789012", Region: "europe-west10", Resource: "mini-orca-archives", Principal: "archive-agent@example-project.iam.gserviceaccount.com", CredentialFileEnv: "MINI_ORCA_CLOUD_KEY", RetentionDays: 30}}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadFromYAML(path)
	if err != nil || len(loaded.Integrations.Cloud) != 1 || loaded.Integrations.Cloud[0] != cfg.Integrations.Cloud[0] {
		t.Fatalf("cloud YAML did not round trip: %+v %v", loaded, err)
	}
	unknown := strings.Replace(string(data), "credential_file_env:", "automatic_upload: true\n          credential_file_env:", 1)
	if unknown == string(data) {
		t.Fatal("fixture did not replace cloud config")
	}
	if err := os.WriteFile(path, []byte(unknown), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFromYAML(path); err == nil || !strings.Contains(err.Error(), "automatic_upload") {
		t.Fatalf("unknown cloud field: %v", err)
	}
}

func TestCloudDestinationsAreExplicitAndBounded(t *testing.T) {
	base := CloudTarget{ID: "archive", Service: "gcs_artifact", Project: "123456789012", Region: "us-central1", Resource: "mini-orca-archives", Principal: "archive-agent@example-project.iam.gserviceaccount.com", CredentialFileEnv: "MINI_ORCA_CLOUD_KEY", RetentionDays: 30}
	valid, err := ResolveIntegrations(IntegrationsConfig{Cloud: []CloudTarget{base}})
	if err != nil || len(valid.Cloud) != 1 || valid.Cloud[0] != base {
		t.Fatalf("resolve: %+v %v", valid, err)
	}
	for _, region := range []string{"europe-west10", "europe-west12", "northamerica-northeast1", "asia-southeast2"} {
		regional := base
		regional.Region = region
		if err := regional.Validate(); err != nil {
			t.Fatalf("valid region %s rejected: %v", region, err)
		}
	}
	for _, change := range []func(*CloudTarget){
		func(c *CloudTarget) { c.Service = "vertex_memory" }, func(c *CloudTarget) { c.Project = "other/123" }, func(c *CloudTarget) { c.Region = "global" }, func(c *CloudTarget) { c.Resource = "bucket/path" }, func(c *CloudTarget) { c.Principal = "operator@example.com" }, func(c *CloudTarget) { c.CredentialFileEnv = "literal-private-key" }, func(c *CloudTarget) { c.RetentionDays = 0 }, func(c *CloudTarget) { c.RetentionDays = 366 },
	} {
		bad := base
		change(&bad)
		if _, err := ResolveIntegrations(IntegrationsConfig{Cloud: []CloudTarget{bad}}); err == nil {
			t.Fatalf("invalid target accepted: %+v", bad)
		}
	}
	if _, err := ResolveIntegrations(IntegrationsConfig{Cloud: []CloudTarget{base, base}}); err == nil {
		t.Fatal("duplicate target accepted")
	}
	base.Service, base.Resource = "vertex_session", "12345"
	if err := base.Validate(); err != nil {
		t.Fatal(err)
	}
	base.Resource = "projects/123456/locations/us-central1/reasoningEngines/12345"
	if err := base.Validate(); err == nil {
		t.Fatal("resource escaped configured project/location")
	}
}
