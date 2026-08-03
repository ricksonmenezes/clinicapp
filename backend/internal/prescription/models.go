package prescription

import (
	"errors"
	"time"
)

// Prescription is authored by the consultant identified by ConsultantID —
// always resolved from the authenticated caller's own consultant profile,
// never a caller-supplied id, so a clinician can't issue an Rx under another
// clinician's name. SessionID is optional context (which visit prompted the
// Rx); when set, it must belong to the same patient.
type Prescription struct {
	ID           string
	SessionID    *string
	ConsultantID string
	PatientID    string
	Content      string
	IssuedAt     time.Time
	PDFPath      *string
}

var (
	ErrNotFound = errors.New("prescription not found")

	ErrConsultantProfileMissing = errors.New("authenticated user has no consultant profile")
	ErrPatientNotFound          = errors.New("patient not found")
	ErrSessionNotFound          = errors.New("session not found")
	ErrSessionPatientMismatch   = errors.New("session does not belong to this patient")
	ErrValidation               = errors.New("patient_id and content are required")
)
