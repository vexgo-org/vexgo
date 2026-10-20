package idgen

import (
	"github.com/google/uuid"
)

type Generator interface {
	New() (uuid.UUID, error)
}

type UUIDv7Generator struct{}

func NewUUIDv7Generator() *UUIDv7Generator {
	return &UUIDv7Generator{}
}

func (g *UUIDv7Generator) New() (uuid.UUID, error) {
	return uuid.NewV7()
}
