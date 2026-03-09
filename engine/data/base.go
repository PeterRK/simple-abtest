package data

import (
	"context"

	"github.com/peterrk/simple-abtest/engine/core"
)

type Application struct {
	AccessToken string            `json:"token,omitempty"`
	Experiments []core.Experiment `json:"exp,omitempty"`
}

// Source fetches experiment data from a storage backend.
type Source interface {
	Fetch(ctx context.Context) (map[uint32]Application, error)
	Close()
}
