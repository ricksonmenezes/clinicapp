package invoice

import (
	"errors"
	"time"
)

// Invoice always has exactly one of SessionID/PackageID set — enforced by a
// DB CHECK constraint (migration 017) as well as here, since it's a real
// financial-record invariant, not just an API nicety.
type Invoice struct {
	ID        string
	SessionID *string
	PackageID *string
	PatientID string
	Total     float64
	IssuedAt  time.Time
	PDFPath   *string
}

// TemplatePlaceholder is an admin-editable key/value pair substituted into
// generated invoice PDFs (clinic_name, clinic_address, footer_text) — never
// hardcoded in the PDF template, per CLAUDE.md's hard constraints.
type TemplatePlaceholder struct {
	ID        string
	Key       string
	Value     string
	UpdatedAt time.Time
}

const (
	PlaceholderClinicName    = "clinic_name"
	PlaceholderClinicAddress = "clinic_address"
	PlaceholderFooterText    = "footer_text"
)

var (
	ErrNotFound               = errors.New("invoice not found")
	ErrSessionNotFound        = errors.New("session not found")
	ErrPatientPackageNotFound = errors.New("patient package not found")
	ErrValidation             = errors.New("exactly one of session_id or package_id is required")
	ErrPlaceholderKeyRequired = errors.New("key is required")
)
