package session

import (
	"errors"
	"time"
)

// Commission resolution sources, in priority order — the first one that
// applies wins. Snapshotted onto the session's CommissionSnapshot so
// historical reporting is immune to later config changes.
const (
	ResolutionSessionOverride   = "session_override"
	ResolutionServiceOverride   = "service_override"
	ResolutionConsultantDefault = "consultant_default"
)

type Session struct {
	ID                 string
	PatientID          string
	ServiceID          string
	PatientPackageID   *string
	ScheduledAt        time.Time
	CompletedAt        *time.Time
	ConsultantID       *string
	Notes              *string
	AttendantIDs       []string
	CommissionSnapshot *CommissionSnapshot
	CreatedAt          time.Time
}

// CommissionSnapshot is nil when a session has no ConsultantID (an
// attendant-only service has nothing to calculate commission on).
type CommissionSnapshot struct {
	ID               string
	SessionID        string
	ConsultantID     string
	CommissionRate   float64
	ResolutionSource string
	ClinicAmount     float64
	ConsultantAmount float64
	CreatedAt        time.Time
}

var (
	ErrNotFound = errors.New("session not found")

	// FK validation errors — a foreign id given in the request body doesn't
	// exist or doesn't satisfy a business rule (400), matching the
	// ErrXNotFound convention used by patient/consultant/promo.
	ErrPatientNotFound        = errors.New("patient not found")
	ErrServiceNotFound        = errors.New("service not found")
	ErrConsultantNotFound     = errors.New("consultant not found")
	ErrAttendantNotFound      = errors.New("attendant not found")
	ErrPatientPackageNotFound = errors.New("patient package not found")

	ErrPatientPackageMismatch    = errors.New("patient_package does not belong to this patient")
	ErrNoSessionsRemaining       = errors.New("patient package has no sessions remaining")
	ErrConsultantRequired        = errors.New("this service requires a consultant")
	ErrInvalidCommissionOverride = errors.New("commission_override must be between 0 and 100")
	ErrValidation                = errors.New("patient_id, service_id, and scheduled_at are required")
)
