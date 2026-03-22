package inference

import (
	llmctx "github.com/quarkloop/agent/pkg/context"
)

// NewUserMessage creates a user-authored text message for the context.
func NewUserMessage(tc llmctx.TokenComputer, idGen llmctx.IDGenerator, author string, text string) (*llmctx.Message, error) {
	id, err := idGen.Next()
	if err != nil {
		return nil, err
	}
	authorID, _ := llmctx.NewAuthorID(author)
	return llmctx.NewTextMessage(id, authorID, llmctx.UserAuthor, text, tc)
}

// NewAgentMessage creates an agent-authored text message from an LLM response.
func NewAgentMessage(tc llmctx.TokenComputer, idGen llmctx.IDGenerator, author string, text string) (*llmctx.Message, error) {
	id, err := idGen.Next()
	if err != nil {
		return nil, err
	}
	authorID, _ := llmctx.NewAuthorID(author)
	return llmctx.NewTextMessage(id, authorID, llmctx.AgentAuthor, text, tc)
}
