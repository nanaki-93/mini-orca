package client

import "time"

type CloudTarget struct {
	ID                string `json:"id"`
	Service           string `json:"service"`
	Project           string `json:"project"`
	Region            string `json:"region"`
	Resource          string `json:"resource"`
	Principal         string `json:"principal"`
	CredentialFileEnv string `json:"credential_file_env"`
	RetentionDays     int    `json:"retention_days"`
}
type CloudSelection struct {
	ProjectIdentity
	TargetID string `json:"target_id"`
	Kind     string `json:"kind"`
	SourceID string `json:"source_id"`
}
type CloudPreview struct {
	CloudSelection
	ID             string      `json:"id"`
	Target         CloudTarget `json:"target"`
	Configuration  string      `json:"configuration"`
	SourceRevision string      `json:"source_revision"`
	SourceVersion  int64       `json:"source_version"`
	Hash           string      `json:"hash"`
	Size           int         `json:"size"`
}
type CloudCopy struct {
	ProjectIdentity
	ID             string      `json:"id"`
	Version        int64       `json:"version"`
	Target         CloudTarget `json:"target"`
	Configuration  string      `json:"configuration"`
	Kind           string      `json:"kind"`
	SourceID       string      `json:"source_id"`
	SourceVersion  int64       `json:"source_version"`
	SourceRevision string      `json:"source_revision"`
	Hash           string      `json:"hash"`
	Size           int         `json:"size"`
	State          string      `json:"state"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
	CleanupAt      time.Time   `json:"cleanup_at"`
	LeaseUntil     time.Time   `json:"lease_until"`
}
type CloudDestination struct {
	Target        CloudTarget `json:"target"`
	Configuration string      `json:"configuration"`
	State         string      `json:"state"`
}
type CloudCatalog struct {
	ProjectIdentity
	Destinations []CloudDestination `json:"destinations"`
	Copies       []CloudCopy        `json:"copies"`
}
type CloudControl struct {
	ProjectIdentity
	Action               string        `json:"action"`
	ID                   string        `json:"id"`
	Version              int64         `json:"version"`
	Preview              *CloudPreview `json:"preview,omitempty"`
	Confirm              bool          `json:"confirm"`
	ConfirmSourceSharing bool          `json:"confirm_source_sharing"`
}
