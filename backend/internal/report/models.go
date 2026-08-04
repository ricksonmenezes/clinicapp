package report

import "time"

// RevenuePoint is total invoiced revenue for one bucket of the requested
// group_by (day/week/month), from internal/invoice's issued_at.
type RevenuePoint struct {
	Period time.Time
	Total  float64
}

// CommissionPayout is what one consultant earned in the requested date
// range, aggregated from internal/session's commission snapshots — the
// payroll-facing report. ClinicAmount is the clinic's matching cut of the
// same sessions, included for reconciliation.
type CommissionPayout struct {
	ConsultantID     string
	ConsultantName   string
	ConsultantAmount float64
	ClinicAmount     float64
	SessionCount     int64
}

// ServicePopularity is how many sessions of a given service were scheduled
// in the requested date range, most-booked first.
type ServicePopularity struct {
	ServiceID    string
	ServiceName  string
	SessionCount int64
}

// BookingVolumePoint is the count of sessions scheduled in one bucket of the
// requested group_by (day/week/month) — every session, not just
// self-service bookings, matching internal/session.List's own scope.
type BookingVolumePoint struct {
	Period time.Time
	Count  int64
}
