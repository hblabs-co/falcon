package interfaces

// Inspector scrapes a project page from a specific source and persists the result.
// Each source (e.g. freelance.de, upwork.com) has its own implementation.
type Inspector interface {
	Inspect() (Project, error)
}
