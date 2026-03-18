package message

import "fmt"

// AudioMediaType is the MIME type of an audio clip.
type AudioMediaType string

const (
	AudioMediaTypeMP3  AudioMediaType = "audio/mp3"
	AudioMediaTypeWAV  AudioMediaType = "audio/wav"
	AudioMediaTypeWEBM AudioMediaType = "audio/webm"
	AudioMediaTypeOGG  AudioMediaType = "audio/ogg"
)

// AudioPayload carries an audio clip for speech-to-text or audio-reasoning agents.
type AudioPayload struct {
	// MediaType is the audio MIME type.
	MediaType AudioMediaType `json:"media_type"`
	// Source tells adapters how to interpret Data.
	Source ImageSource `json:"source"` // reuses ImageSource (base64 / url / file)
	// Data is base64 bytes, a URL, or a file path depending on Source.
	Data string `json:"data,omitempty"`
	// Transcript is an optional pre-computed transcription.
	// Used for token counting and for adapters without native audio support.
	Transcript string `json:"transcript,omitempty"`
	// DurationSeconds is the clip length, if known.
	DurationSeconds float64 `json:"duration_seconds,omitempty"`
}

func init() { RegisterPayloadFactory(AudioType, func() Payload { return &AudioPayload{} }) }

func (p AudioPayload) Kind() MessageType { return AudioType }
func (p AudioPayload) sealPayload()      {}

func (p AudioPayload) TextRepresentation() string {
	if p.Transcript != "" {
		return fmt.Sprintf("[audio:%s transcript] %s", p.MediaType, p.Transcript)
	}
	return fmt.Sprintf("[audio source=%s media_type=%s duration=%.1fs]",
		p.Source, p.MediaType, p.DurationSeconds)
}

// LLMText provides the transcript (preferred) or a label.
func (p AudioPayload) LLMText() string {
	if p.Transcript != "" {
		return fmt.Sprintf("[Audio transcript: %s]", p.Transcript)
	}
	return fmt.Sprintf("[Audio clip (%.1fs)]", p.DurationSeconds)
}

// UserText shows the user a caption or duration.
func (p AudioPayload) UserText() string {
	if p.Transcript != "" {
		return p.Transcript
	}
	return fmt.Sprintf("🎙 Audio clip (%.1fs)", p.DurationSeconds)
}

// DevText returns full structural detail.
func (p AudioPayload) DevText() string {
	return fmt.Sprintf("[audio media_type=%s source=%s duration=%.1fs transcript_len=%d]",
		p.MediaType, p.Source, p.DurationSeconds, len(p.Transcript))
}
