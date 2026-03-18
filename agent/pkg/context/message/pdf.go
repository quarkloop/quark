package message

import "fmt"

// PDFPayload carries a PDF document, typically used for RAG (retrieval-augmented
// generation) or direct document-QA use cases.
type PDFPayload struct {
	// Filename is the original file name, used for display and audit.
	Filename string `json:"filename"`
	// Source tells adapters how to interpret Data.
	Source ImageSource `json:"source"` // reuses ImageSource (base64 / url / file)
	// Data is base64-encoded PDF bytes, a URL, or a file path.
	Data string `json:"data,omitempty"`
	// ExtractedText is a pre-computed plain-text rendition.
	// Used for token counting and for text-only LLMs.
	ExtractedText string `json:"extracted_text,omitempty"`
	// PageCount is the number of pages, if known.
	PageCount int `json:"page_count,omitempty"`
}

func init() { RegisterPayloadFactory(PDFType, func() Payload { return &PDFPayload{} }) }

func (p PDFPayload) Kind() MessageType { return PDFType }
func (p PDFPayload) sealPayload()      {}

func (p PDFPayload) TextRepresentation() string {
	if p.ExtractedText != "" {
		return fmt.Sprintf("[PDF: %s]\n%s", p.Filename, p.ExtractedText)
	}
	return fmt.Sprintf("[PDF: %s, %d pages]", p.Filename, p.PageCount)
}

// LLMText provides the extracted text (preferred) or a label.
func (p PDFPayload) LLMText() string {
	if p.ExtractedText != "" {
		return fmt.Sprintf("[Document: %s]\n%s", p.Filename, p.ExtractedText)
	}
	return fmt.Sprintf("[Document: %s (%d pages)]", p.Filename, p.PageCount)
}

// UserText shows the user a friendly label.
func (p PDFPayload) UserText() string {
	return fmt.Sprintf("📄 %s (%d pages)", p.Filename, p.PageCount)
}

// DevText returns full structural detail.
func (p PDFPayload) DevText() string {
	return fmt.Sprintf("[pdf filename=%q source=%s pages=%d extracted_len=%d]",
		p.Filename, p.Source, p.PageCount, len(p.ExtractedText))
}
