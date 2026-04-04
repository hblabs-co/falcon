package models

// ProjectEvent is published to "project.created" or "project.updated"
// by falcon-scout whenever a project is first detected or has changed.
type ProjectEvent struct {
	ProjectID  string `json:"project_id"`
	Platform   string `json:"platform"`
	PlatformID string `json:"platform_id"`
	Title      string `json:"title"`
}
