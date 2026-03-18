package message

import "fmt"

// ImageMediaType is the MIME type of an image payload.
type ImageMediaType string

const (
	ImageMediaTypeJPEG ImageMediaType = "image/jpeg"
	ImageMediaTypePNG  ImageMediaType = "image/png"
	ImageMediaTypeGIF  ImageMediaType = "image/gif"
	ImageMediaTypeWEBP ImageMediaType = "image/webp"
)

// ImageSource describes how image data is supplied.
type ImageSource string

const (
	// ImageSourceBase64 means Data contains inline base64-encoded bytes.
	ImageSourceBase64 ImageSource = "base64"
	// ImageSourceURL means Data is a remote HTTP/HTTPS URL.
	ImageSourceURL ImageSource = "url"
	// ImageSourceFile means Data is a local file path (agent-side only; never sent to LLM).
	ImageSourceFile ImageSource = "file"
)

// ImagePayload carries a single image and an optional natural-language description.
//
// Vision-capable adapters type-assert to ImagePayload and use the Data field
// directly.  Text-only adapters fall back to Description.
type ImagePayload struct {
	// MediaType is the MIME type (e.g. "image/jpeg").
	MediaType ImageMediaType `json:"media_type"`
	// Source tells adapters how to interpret Data.
	Source ImageSource `json:"source"`
	// Data is base64 bytes, a URL, or a local file path depending on Source.
	Data string `json:"data"`
	// Description is an optional human/agent caption.
	Description string `json:"description,omitempty"`
	// Width and Height in pixels (optional; helps with token estimation).
	Width  int `json:"width,omitempty"`
	Height int `json:"height,omitempty"`
}

func init() { RegisterPayloadFactory(ImageType, func() Payload { return &ImagePayload{} }) }

func (p ImagePayload) Kind() MessageType { return ImageType }
func (p ImagePayload) sealPayload()      {}

func (p ImagePayload) TextRepresentation() string {
	if p.Description != "" {
		return fmt.Sprintf("[image:%s] %s", p.MediaType, p.Description)
	}
	return fmt.Sprintf("[image source=%s media_type=%s]", p.Source, p.MediaType)
}

// LLMText provides a description for text-only models.
// Vision-capable adapters use the raw Data field instead.
func (p ImagePayload) LLMText() string {
	if p.Description != "" {
		return fmt.Sprintf("[Image: %s]", p.Description)
	}
	return fmt.Sprintf("[Image (%s)]", p.MediaType)
}

// UserText shows the user a caption or a brief label.
func (p ImagePayload) UserText() string {
	if p.Description != "" {
		return p.Description
	}
	return fmt.Sprintf("[Image (%s, %dx%d)]", p.MediaType, p.Width, p.Height)
}

// DevText returns full structural detail for developer tooling.
func (p ImagePayload) DevText() string {
	return fmt.Sprintf("[image media_type=%s source=%s w=%d h=%d] %s",
		p.MediaType, p.Source, p.Width, p.Height, p.Description)
}
