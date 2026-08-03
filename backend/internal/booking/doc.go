// Package booking implements the customer-facing self-booking flow
// (PLAN.md Module 8): calendar slot availability and appointment booking.
// It's a thin layer over internal/session — a booking is a session, created
// through session.Service so it gets the exact same validation and
// commission-resolution pipeline as a backoffice-created session.
package booking
