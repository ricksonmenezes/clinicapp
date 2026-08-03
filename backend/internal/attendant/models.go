package attendant

import (
	"errors"
	"time"
)

type Attendant struct {
	ID        string
	UserID    string
	FullName  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

var (
	ErrNotFound      = errors.New("attendant not found")
	ErrUserNotFound  = errors.New("user not found")
	ErrAlreadyExists = errors.New("attendant profile already exists for this user")
	ErrInvalidRole   = errors.New("user must have the attendant role")
	ErrValidation    = errors.New("full_name is required")
)
