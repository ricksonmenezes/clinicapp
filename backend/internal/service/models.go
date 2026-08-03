package service

import (
	"errors"
	"time"
)

type Service struct {
	ID                 string
	Name               string
	Description        *string
	Price              float64
	RequiresConsultant bool
	Active             bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

var (
	ErrNotFound   = errors.New("service not found")
	ErrValidation = errors.New("name is required and price must be >= 0")
)
