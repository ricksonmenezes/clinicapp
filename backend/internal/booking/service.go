package booking

import (
	"context"
	"errors"
	"fmt"
	"time"

	"clinicapp/backend/internal/auth"
	"clinicapp/backend/internal/consultant"
	"clinicapp/backend/internal/mailer"
	"clinicapp/backend/internal/patient"
	"clinicapp/backend/internal/service"
	"clinicapp/backend/internal/session"
)

// Fixed clinic operating calendar (UTC). Not admin-configurable — PLAN.md's
// customer portal scope (Module 8) never mentions configurable hours, and
// nothing else in the schema stores them, so this stays a package constant
// like other modules' MVP defaults rather than inventing a second admin
// settings surface alongside the invoice placeholder table. Revisit if a
// real multi-day/branch schedule is ever needed.
const (
	openHour    = 9
	closeHour   = 17
	slotMinutes = 30
)

type Service struct {
	sessionSvc     *session.Service
	sessionRepo    *session.Repository
	serviceRepo    *service.Repository
	consultantRepo *consultant.Repository
	patientRepo    *patient.Repository
	authRepo       *auth.Repository
	mailer         mailer.Mailer
}

func NewService(
	sessionSvc *session.Service,
	sessionRepo *session.Repository,
	serviceRepo *service.Repository,
	consultantRepo *consultant.Repository,
	patientRepo *patient.Repository,
	authRepo *auth.Repository,
	m mailer.Mailer,
) *Service {
	return &Service{
		sessionSvc:     sessionSvc,
		sessionRepo:    sessionRepo,
		serviceRepo:    serviceRepo,
		consultantRepo: consultantRepo,
		patientRepo:    patientRepo,
		authRepo:       authRepo,
		mailer:         m,
	}
}

// Availability returns every clinic slot on the given date (YYYY-MM-DD,
// interpreted in UTC) for a service, flagged available/unavailable.
//
// Clinic capacity per (service, slot) is 1 — the schema has no
// consultant-service capability mapping to model "which consultants can
// perform this service", so rather than invent one, a slot holds at most
// one booking of a given service regardless of how many consultants exist.
// For services that require a consultant, a slot additionally needs at
// least one consultant with no session (of any service) at that exact time
// — otherwise Book couldn't find anyone to auto-assign.
func (s *Service) Availability(ctx context.Context, serviceID, date string) ([]Slot, error) {
	svc, err := s.serviceRepo.GetByID(ctx, serviceID)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return nil, ErrServiceNotFound
		}
		return nil, err
	}

	day, err := time.Parse("2006-01-02", date)
	if err != nil {
		return nil, ErrInvalidDate
	}

	starts := daySlots(day)
	if len(starts) == 0 {
		return []Slot{}, nil
	}

	occupied, err := s.sessionRepo.ListOccupancyInRange(ctx, starts[0], starts[len(starts)-1].Add(slotMinutes*time.Minute))
	if err != nil {
		return nil, err
	}

	var consultants []*consultant.Consultant
	if svc.RequiresConsultant {
		consultants, err = s.consultantRepo.List(ctx)
		if err != nil {
			return nil, err
		}
	}

	now := time.Now().UTC()
	slots := make([]Slot, 0, len(starts))
	for _, start := range starts {
		if start.Before(now) {
			continue
		}
		slots = append(slots, Slot{
			ScheduledAt: start,
			Available:   slotAvailable(svc, consultants, occupied, start),
		})
	}
	return slots, nil
}

type CreateBookingInput struct {
	ServiceID   string
	ScheduledAt time.Time
}

// Book creates a session on behalf of the authenticated patient — always
// the caller's own patient profile, never a caller-supplied patient_id,
// same authorship-security pattern as prescription.Service.Create. If the
// service requires a consultant, one is auto-assigned from whoever has no
// conflicting session at that exact time: PLAN.md's booking flow is
// "choose service -> choose date/time -> confirm", the patient never picks
// a consultant. Availability is re-checked here (not trusted from a prior
// GET /availability call) to guard against it going stale between the
// calendar view and the booking request.
func (s *Service) Book(ctx context.Context, callerUserID string, in CreateBookingInput) (*session.Session, error) {
	pat, err := s.patientRepo.GetByUserID(ctx, callerUserID)
	if err != nil {
		if errors.Is(err, patient.ErrNotFound) {
			return nil, ErrPatientProfileMissing
		}
		return nil, err
	}

	svc, err := s.serviceRepo.GetByID(ctx, in.ServiceID)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return nil, ErrServiceNotFound
		}
		return nil, err
	}
	if !svc.Active {
		return nil, ErrServiceInactive
	}

	scheduledAt := in.ScheduledAt.UTC()
	if !isValidSlot(scheduledAt) || scheduledAt.Before(time.Now().UTC()) {
		return nil, ErrInvalidSlot
	}

	dayStart := time.Date(scheduledAt.Year(), scheduledAt.Month(), scheduledAt.Day(), 0, 0, 0, 0, time.UTC)
	occupied, err := s.sessionRepo.ListOccupancyInRange(ctx, dayStart, dayStart.Add(24*time.Hour))
	if err != nil {
		return nil, err
	}
	for _, o := range occupied {
		if o.ServiceID == svc.ID && o.ScheduledAt.Equal(scheduledAt) {
			return nil, ErrSlotUnavailable
		}
	}

	var consultantID *string
	if svc.RequiresConsultant {
		consultants, err := s.consultantRepo.List(ctx)
		if err != nil {
			return nil, err
		}
		free := freeConsultant(consultants, occupied, scheduledAt)
		if free == nil {
			return nil, ErrSlotUnavailable
		}
		consultantID = &free.ID
	}

	sess, err := s.sessionSvc.Create(ctx, session.CreateInput{
		PatientID:    pat.ID,
		ServiceID:    svc.ID,
		ScheduledAt:  scheduledAt,
		ConsultantID: consultantID,
	})
	if err != nil {
		return nil, err
	}

	if err := s.sendConfirmation(ctx, callerUserID, svc, sess); err != nil {
		return nil, err
	}

	return sess, nil
}

func (s *Service) sendConfirmation(ctx context.Context, callerUserID string, svc *service.Service, sess *session.Session) error {
	user, err := s.authRepo.GetUserByID(ctx, callerUserID)
	if err != nil {
		return err
	}
	return s.mailer.Send(ctx, mailer.Message{
		To:      user.Email,
		Subject: "Your appointment is confirmed",
		Body: fmt.Sprintf(
			"Your booking for %s on %s is confirmed.",
			svc.Name, sess.ScheduledAt.Format(time.RFC1123),
		),
	})
}

// ListOwn returns the authenticated patient's own booking history.
func (s *Service) ListOwn(ctx context.Context, callerUserID string) ([]*session.Session, error) {
	pat, err := s.patientRepo.GetByUserID(ctx, callerUserID)
	if err != nil {
		if errors.Is(err, patient.ErrNotFound) {
			return nil, ErrPatientProfileMissing
		}
		return nil, err
	}
	return s.sessionSvc.List(ctx, session.ListFilter{PatientID: pat.ID})
}

// GetOwn returns a single booking, but only if it belongs to the
// authenticated patient — a session id that exists but belongs to someone
// else reports session.ErrNotFound (404) rather than 403, so a patient
// can't use this to probe for the existence of another patient's
// appointments.
func (s *Service) GetOwn(ctx context.Context, callerUserID, sessionID string) (*session.Session, error) {
	pat, err := s.patientRepo.GetByUserID(ctx, callerUserID)
	if err != nil {
		if errors.Is(err, patient.ErrNotFound) {
			return nil, ErrPatientProfileMissing
		}
		return nil, err
	}
	sess, err := s.sessionSvc.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if sess.PatientID != pat.ID {
		return nil, session.ErrNotFound
	}
	return sess, nil
}

// daySlots returns every slot start time on the given date, or nil if the
// clinic is closed that day (Sundays).
func daySlots(day time.Time) []time.Time {
	if day.Weekday() == time.Sunday {
		return nil
	}
	var starts []time.Time
	for h := openHour; h < closeHour; h++ {
		for m := 0; m < 60; m += slotMinutes {
			starts = append(starts, time.Date(day.Year(), day.Month(), day.Day(), h, m, 0, 0, time.UTC))
		}
	}
	return starts
}

func isValidSlot(t time.Time) bool {
	if t.Second() != 0 || t.Nanosecond() != 0 || t.Minute()%slotMinutes != 0 {
		return false
	}
	if t.Hour() < openHour || t.Hour() >= closeHour {
		return false
	}
	return t.Weekday() != time.Sunday
}

func slotAvailable(svc *service.Service, consultants []*consultant.Consultant, occupied []session.SlotOccupancy, start time.Time) bool {
	for _, o := range occupied {
		if o.ServiceID == svc.ID && o.ScheduledAt.Equal(start) {
			return false
		}
	}
	if !svc.RequiresConsultant {
		return true
	}
	return freeConsultant(consultants, occupied, start) != nil
}

func freeConsultant(consultants []*consultant.Consultant, occupied []session.SlotOccupancy, start time.Time) *consultant.Consultant {
	for _, c := range consultants {
		busy := false
		for _, o := range occupied {
			if o.ConsultantID != nil && *o.ConsultantID == c.ID && o.ScheduledAt.Equal(start) {
				busy = true
				break
			}
		}
		if !busy {
			return c
		}
	}
	return nil
}
