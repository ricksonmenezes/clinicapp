package session

import (
	"context"
	"errors"
	"math"
	"time"

	"clinicapp/backend/internal/attendant"
	"clinicapp/backend/internal/consultant"
	"clinicapp/backend/internal/patient"
	"clinicapp/backend/internal/promo"
	"clinicapp/backend/internal/service"
)

type Service struct {
	repo               *Repository
	patientRepo        *patient.Repository
	serviceRepo        *service.Repository
	consultantRepo     *consultant.Repository
	attendantRepo      *attendant.Repository
	patientPackageRepo *promo.PatientPackageRepository
}

func NewService(repo *Repository, patientRepo *patient.Repository, serviceRepo *service.Repository, consultantRepo *consultant.Repository, attendantRepo *attendant.Repository, patientPackageRepo *promo.PatientPackageRepository) *Service {
	return &Service{
		repo:               repo,
		patientRepo:        patientRepo,
		serviceRepo:        serviceRepo,
		consultantRepo:     consultantRepo,
		attendantRepo:      attendantRepo,
		patientPackageRepo: patientPackageRepo,
	}
}

type CreateInput struct {
	PatientID          string
	ServiceID          string
	PatientPackageID   *string
	ScheduledAt        time.Time
	CompletedAt        *time.Time
	ConsultantID       *string
	Notes              *string
	AttendantIDs       []string
	CommissionOverride *float64
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*Session, error) {
	if in.PatientID == "" || in.ServiceID == "" || in.ScheduledAt.IsZero() {
		return nil, ErrValidation
	}
	if in.CommissionOverride != nil && (*in.CommissionOverride < 0 || *in.CommissionOverride > 100) {
		return nil, ErrInvalidCommissionOverride
	}

	if _, err := s.patientRepo.GetByID(ctx, in.PatientID); err != nil {
		if errors.Is(err, patient.ErrNotFound) {
			return nil, ErrPatientNotFound
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

	if svc.RequiresConsultant && in.ConsultantID == nil {
		return nil, ErrConsultantRequired
	}

	var cons *consultant.Consultant
	if in.ConsultantID != nil {
		cons, err = s.consultantRepo.GetByID(ctx, *in.ConsultantID)
		if err != nil {
			if errors.Is(err, consultant.ErrNotFound) {
				return nil, ErrConsultantNotFound
			}
			return nil, err
		}
	}

	if in.PatientPackageID != nil {
		pp, err := s.patientPackageRepo.GetByID(ctx, *in.PatientPackageID)
		if err != nil {
			if errors.Is(err, promo.ErrPatientPackageNotFound) {
				return nil, ErrPatientPackageNotFound
			}
			return nil, err
		}
		if pp.PatientID != in.PatientID {
			return nil, ErrPatientPackageMismatch
		}
		if pp.SessionsRemaining <= 0 {
			return nil, ErrNoSessionsRemaining
		}
	}

	for _, attendantID := range in.AttendantIDs {
		if _, err := s.attendantRepo.GetByID(ctx, attendantID); err != nil {
			if errors.Is(err, attendant.ErrNotFound) {
				return nil, ErrAttendantNotFound
			}
			return nil, err
		}
	}

	var snapshot *CommissionSnapshot
	if cons != nil {
		rate, source, err := s.resolveCommission(ctx, *in.ConsultantID, in.ServiceID, in.CommissionOverride, cons.DefaultCommission)
		if err != nil {
			return nil, err
		}
		consultantAmount := round2(svc.Price * rate / 100)
		clinicAmount := round2(svc.Price - consultantAmount)
		snapshot = &CommissionSnapshot{
			ConsultantID:     *in.ConsultantID,
			CommissionRate:   rate,
			ResolutionSource: source,
			ClinicAmount:     clinicAmount,
			ConsultantAmount: consultantAmount,
		}
	}

	return s.repo.Create(ctx, CreateParams{
		PatientID:        in.PatientID,
		ServiceID:        in.ServiceID,
		PatientPackageID: in.PatientPackageID,
		ScheduledAt:      in.ScheduledAt,
		CompletedAt:      in.CompletedAt,
		ConsultantID:     in.ConsultantID,
		Notes:            in.Notes,
		AttendantIDs:     in.AttendantIDs,
		Snapshot:         snapshot,
	})
}

// resolveCommission implements PLAN.md's resolution order: session-level
// override wins, then service-level override, then consultant default.
func (s *Service) resolveCommission(ctx context.Context, consultantID, serviceID string, override *float64, defaultCommission float64) (rate float64, source string, err error) {
	if override != nil {
		return *override, ResolutionSessionOverride, nil
	}
	sc, err := s.consultantRepo.GetServiceCommission(ctx, consultantID, serviceID)
	if err != nil {
		if errors.Is(err, consultant.ErrServiceCommissionNotFound) {
			return defaultCommission, ResolutionConsultantDefault, nil
		}
		return 0, "", err
	}
	return sc.Commission, ResolutionServiceOverride, nil
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func (s *Service) Get(ctx context.Context, id string) (*Session, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) List(ctx context.Context, filter ListFilter) ([]*Session, error) {
	return s.repo.List(ctx, filter)
}

// CommissionHistory returns a consultant's commission snapshots, most
// recent first, after confirming the consultant exists. Unlike
// ErrConsultantNotFound (used when a body-supplied consultant_id fails
// validation, i.e. 400), a missing consultant here is a path-param lookup —
// consultant.ErrNotFound propagates as-is so statusForError can 404 it, the
// same as consultant.Service.ListServiceCommissions does for its own
// /consultants/{id}/... sub-resource.
func (s *Service) CommissionHistory(ctx context.Context, consultantID string) ([]*CommissionSnapshot, error) {
	if _, err := s.consultantRepo.GetByID(ctx, consultantID); err != nil {
		return nil, err
	}
	return s.repo.ListCommissionHistory(ctx, consultantID)
}
