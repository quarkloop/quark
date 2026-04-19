package message

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

// List handles GET /v1/sessions/:session_id/messages
func (h *Handler) List(c *gin.Context) {
	sessionID := c.Param("session_id")
	if !h.sessions.Has(sessionID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}
	c.JSON(http.StatusOK, h.sessions.GetMessages(sessionID))
}

// Send handles POST /v1/sessions/:session_id/messages
func (h *Handler) Send(c *gin.Context) {
	sessionID := c.Param("session_id")

	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !h.sessions.Has(sessionID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	resp := make(chan StreamMessage, 64)
	h.poster.Post(sessionID, req.Content, resp)

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	c.Stream(func(w io.Writer) bool {
		select {
		case msgData, ok := <-resp:
			if !ok {
				return false
			}
			// Always JSON-encode the payload so clients can parse it
			// uniformly regardless of whether Data is a string, map, or
			// struct. Gin's default SSEvent writes strings raw, which
			// breaks round-tripping of tokens containing newlines or
			// forces clients to branch on type per event.
			payload, err := json.Marshal(msgData.Data)
			if err != nil {
				return false
			}
			c.SSEvent(msgData.Type, json.RawMessage(payload))
			return true
		case <-c.Request.Context().Done():
			return false
		}
	})
}

// Stream handles GET /v1/sessions/:session_id/messages/stream
func (h *Handler) Stream(c *gin.Context) {
	sessionID := c.Param("session_id")

	ch := h.sessions.Subscribe(sessionID)
	if ch == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}
	defer h.sessions.Unsubscribe(sessionID, ch)

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	c.Stream(func(w io.Writer) bool {
		select {
		case msg, ok := <-ch:
			if !ok {
				return false
			}
			c.SSEvent("message", msg)
			return true
		case <-c.Request.Context().Done():
			return false
		}
	})
}

// Edit handles PATCH /v1/sessions/:session_id/messages/:message_id
func (h *Handler) Edit(c *gin.Context) {
	c.AbortWithStatus(http.StatusNotImplemented)
}
