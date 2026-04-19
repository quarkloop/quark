package message

import (
	"github.com/gin-gonic/gin"
)

// Poster posts messages to the agent inbox.
type Poster interface {
	Post(sessionID, content string, resp chan StreamMessage)
}

// SessionAccess provides session state for message handlers.
type SessionAccess interface {
	Has(id string) bool
	GetMessages(id string) []Message
	Subscribe(id string) chan Message
	Unsubscribe(id string, ch chan Message)
}

// Handler holds message handler dependencies.
type Handler struct {
	poster   Poster
	sessions SessionAccess
}

// NewHandler creates a new message Handler.
func NewHandler(p Poster, sa SessionAccess) *Handler {
	return &Handler{poster: p, sessions: sa}
}

// RegisterRoutes wires message routes.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("", h.List)
	r.POST("", h.Send)
	r.GET("/stream", h.Stream)
	r.PATCH("/:message_id", h.Edit)
}
