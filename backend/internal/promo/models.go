package promo

import (
	"errors"
	"time"
)

// Package is a bundle of N sessions of a service sold at a package price.
type Package struct {
	ID           string
	ServiceID    string
	Name         string
	SessionCount int
	Price        float64
	Active       bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// PatientPackage is a patient's subscription to a Package. PrincipalConsultant
// is informational only — per PLAN.md, commission always follows whichever
// consultant actually performs each session, not this field.
type PatientPackage struct {
	ID                  string
	PatientID           string
	PackageID           string
	PrincipalConsultant *string
	SessionsRemaining   int
	PurchasedAt         time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

var (
	// ErrPackageNotFound/ErrPatientPackageNotFound: the resource identified by
	// a path param doesn't exist (404).
	ErrPackageNotFound        = errors.New("package not found")
	ErrPatientPackageNotFound = errors.New("patient package not found")

	// ErrServiceNotFound/ErrPatientNotFound/ErrConsultantNotFound/
	// ErrInvalidPackageID: a foreign id given in a request body doesn't exist
	// (400) — matches the ErrUserNotFound convention in patient/consultant/
	// attendant for FK validation.
	ErrServiceNotFound    = errors.New("service not found")
	ErrPatientNotFound    = errors.New("patient not found")
	ErrConsultantNotFound = errors.New("consultant not found")
	ErrInvalidPackageID   = errors.New("package not found")

	ErrValidation = errors.New("name is required, session_count must be > 0, and price must be >= 0")
)
