package cv

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// extractText reads a .docx file from r (of size sz bytes) and returns the
// plain text from the document body, with whitespace normalised.
func extractText(r io.Reader, sz int64) (string, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("read docx: %w", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf), sz)
	if err != nil {
		return "", fmt.Errorf("open docx zip: %w", err)
	}

	for _, f := range zr.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("open document.xml: %w", err)
		}
		defer rc.Close()
		return parseDocumentXML(rc)
	}
	return "", fmt.Errorf("word/document.xml not found in docx")
}

// parseDocumentXML extracts visible text from a Word XML document.
// Handles paragraphs, table rows/cells, line breaks, and xml:space="preserve".
func parseDocumentXML(r io.Reader) (string, error) {
	var sb strings.Builder
	decoder := xml.NewDecoder(r)

	inBody := false
	inT := false         // inside <w:t>
	preserveSpace := false

	// Pending structural whitespace, emitted lazily so trailing whitespace is never written.
	pendingNewline := false
	pendingTab := false

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("parse xml: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "body":
				inBody = true
			case "p", "tr": // paragraph / table row → newline boundary
				if inBody {
					pendingNewline = true
				}
			case "tc": // table cell → tab separator (unless a newline is already pending)
				if inBody && !pendingNewline {
					pendingTab = true
				}
			case "br": // explicit line break inside a paragraph
				if inBody && sb.Len() > 0 {
					sb.WriteByte('\n')
					pendingNewline = false
					pendingTab = false
				}
			case "t": // <w:t> — actual text run content
				if inBody {
					inT = true
					preserveSpace = false
					for _, attr := range t.Attr {
						if attr.Name.Local == "space" && attr.Value == "preserve" {
							preserveSpace = true
						}
					}
				}
			}

		case xml.CharData:
			if !inBody || !inT {
				break
			}
			s := string(t)
			if !preserveSpace {
				s = strings.TrimSpace(s)
			}
			if s == "" {
				break
			}
			// Flush pending structural whitespace before writing text.
			if pendingNewline && sb.Len() > 0 {
				sb.WriteByte('\n')
			} else if pendingTab && sb.Len() > 0 {
				sb.WriteByte('\t')
			} else if sb.Len() > 0 {
				// Add a space between adjacent runs when neither side already has one.
				last := sb.String()[sb.Len()-1]
				if last != ' ' && last != '\n' && last != '\t' && s[0] != ' ' {
					sb.WriteByte(' ')
				}
			}
			pendingNewline = false
			pendingTab = false
			sb.WriteString(s)

		case xml.EndElement:
			switch t.Name.Local {
			case "body":
				inBody = false
			case "t":
				inT = false
			}
		}
	}

	return strings.TrimSpace(sb.String()), nil
}
