package patient

import (
	"errors"
	"time"
)

type Patient struct {
	ID        string
	UserID    string
	FullName  string
	DOB       *time.Time
	Phone     *string
	Notes     *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

var (
	ErrNotFound      = errors.New("patient not found")
	ErrUserNotFound  = errors.New("user not found")
	ErrAlreadyExists = errors.New("patient profile already exists for this user")
	ErrInvalidRole   = errors.New("user must have the patient role")
	ErrValidation    = errors.New("full_name is required")
	ErrInvalidDOB    = errors.New("dob must be in YYYY-MM-DD format")
)
