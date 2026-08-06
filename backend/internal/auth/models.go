package auth

import (
	"errors"
	"time"
)

const (
	RolePatient   = "patient"
	RoleClinician = "clinician"
	RoleAttendant = "attendant"
	RoleAdmin     = "admin"

	StatusUnverified = "unverified"
	StatusActive     = "active"
	StatusSuspended  = "suspended"
)

type User struct {
	ID           string
	Email        string
	PasswordHash string
	Role         string
	Status       string
	// FullName and DateOfBirth are only ever set for staff accounts
	// created via RegisterStaff — captured purely so an admin can look a
	// staff account up by name (GET /users) instead of handling its id
	// directly. Patients keep their own separate full_name/dob on the
	// patients table (set when they complete their profile); these stay
	// nil for patient/self-registered users.
	FullName     *string
	DateOfBirth  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type EmailVerificationToken struct {
	ID        string
	UserID    string
	Token     string
	ExpiresAt time.Time
	UsedAt    *time.Time
}

type RefreshToken struct {
	ID        string
	UserID    string
	Token     string
	ExpiresAt time.Time
	RevokedAt *time.Time
}

type PasswordResetToken struct {
	ID        string
	UserID    string
	Token     string
	ExpiresAt time.Time
	UsedAt    *time.Time
}

var (
	ErrEmailTaken         = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrEmailNotVerified   = errors.New("email not verified")
	ErrUserNotFound       = errors.New("user not found")
	ErrUserSuspended      = errors.New("user suspended")
	ErrTokenInvalid       = errors.New("invalid token")
	ErrTokenExpired       = errors.New("token expired")
	ErrTokenUsed          = errors.New("token already used")
	ErrTokenRevoked       = errors.New("token revoked")
	ErrWeakPassword       = errors.New("password must be at least 8 characters")
	ErrInvalidRole        = errors.New("invalid role")
	ErrFullNameRequired   = errors.New("full name is required")
	ErrInvalidDateOfBirth = errors.New("invalid date of birth")
)
