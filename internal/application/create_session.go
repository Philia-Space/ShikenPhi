package application

import (
	"context"
	"fmt"

	"github.com/philiaspace/shikenphi/internal/domain"
)

// CreateSessionCommand contains the data needed to start an exam.
type CreateSessionCommand struct {
	UserID     string
	Level      string
	TemplateID string
}

// CreateSessionHandler orchestrates exam session creation.
type CreateSessionHandler struct {
	// TODO: inject MondaiPhi client, session repo
}

func NewCreateSessionHandler() *CreateSessionHandler {
	return &CreateSessionHandler{}
}

func (h *CreateSessionHandler) Handle(ctx context.Context, cmd CreateSessionCommand) (*domain.Session, error) {
	// TODO: call MondaiPhi for questions, build units, shuffle, create session
	return nil, fmt.Errorf("not implemented")
}
