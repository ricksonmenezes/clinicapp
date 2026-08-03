package booking

import (
	"errors"
	"time"
)

// Slot is one bookable calendar position on a given day.
type Slot struct {
	ScheduledAt time.Time
	Available   bool
}

var (
	ErrServiceNotFound       = errors.New("service not found")
	ErrServiceInactive       = errors.New("service is not currently offered")
	ErrInvalidDate           = errors.New("date must be in YYYY-MM-DD format")
	ErrInvalidSlot           = errors.New("scheduled_at is not a valid future clinic slot")
	ErrPatientProfileMissing = errors.New("complete your patient profile before booking")
	ErrSlotUnavailable       = errors.New("that slot is no longer available")
)
