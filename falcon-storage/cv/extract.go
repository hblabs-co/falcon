package cv

import "strings"

// isPDF returns true when the filename looks like a PDF upload. Keeps
// the dispatcher in service.go readable and centralises the detection
// in case we add more formats later (.rtf, .txt, etc.).
func isPDF(filename string) bool {
	return strings.HasSuffix(strings.ToLower(filename), ".pdf")
}
