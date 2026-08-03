package invoice

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"clinicapp/backend/internal/patient"
	"clinicapp/backend/internal/promo"
	"clinicapp/backend/internal/service"
	"clinicapp/backend/internal/session"
)

type Service struct {
	repo               *Repository
	placeholderRepo    *PlaceholderRepository
	sessionRepo        *session.Repository
	serviceRepo        *service.Repository
	packageRepo        *promo.PackageRepository
	patientPackageRepo *promo.PatientPackageRepository
	patientRepo        *patient.Repository
	storageDir         string
}

func NewService(
	repo *Repository,
	placeholderRepo *PlaceholderRepository,
	sessionRepo *session.Repository,
	serviceRepo *service.Repository,
	packageRepo *promo.PackageRepository,
	patientPackageRepo *promo.PatientPackageRepository,
	patientRepo *patient.Repository,
	storageDir string,
) *Service {
	return &Service{
		repo:               repo,
		placeholderRepo:    placeholderRepo,
		sessionRepo:        sessionRepo,
		serviceRepo:        serviceRepo,
		packageRepo:        packageRepo,
		patientPackageRepo: patientPackageRepo,
		patientRepo:        patientRepo,
		storageDir:         storageDir,
	}
}

type CreateInput struct {
	// Exactly one of SessionID/PackageID (a patient_package id) must be set.
	SessionID *string
	PackageID *string
}

// Create derives the invoice's patient, line-item description, and total
// from the underlying session or patient_package — the caller only points
// at what's being billed, not what it costs, so an invoice can't drift from
// the record it bills.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Invoice, error) {
	if (in.SessionID == nil) == (in.PackageID == nil) {
		return nil, ErrValidation
	}

	var patientID, lineItem string
	var total float64

	if in.SessionID != nil {
		sess, err := s.sessionRepo.GetByID(ctx, *in.SessionID)
		if err != nil {
			if errors.Is(err, session.ErrNotFound) {
				return nil, ErrSessionNotFound
			}
			return nil, err
		}
		svc, err := s.serviceRepo.GetByID(ctx, sess.ServiceID)
		if err != nil {
			return nil, err
		}
		patientID = sess.PatientID
		lineItem = svc.Name
		total = svc.Price
	} else {
		pp, err := s.patientPackageRepo.GetByID(ctx, *in.PackageID)
		if err != nil {
			if errors.Is(err, promo.ErrPatientPackageNotFound) {
				return nil, ErrPatientPackageNotFound
			}
			return nil, err
		}
		pkg, err := s.packageRepo.GetByID(ctx, pp.PackageID)
		if err != nil {
			return nil, err
		}
		patientID = pp.PatientID
		lineItem = pkg.Name
		total = pkg.Price
	}

	pat, err := s.patientRepo.GetByID(ctx, patientID)
	if err != nil {
		return nil, err
	}

	inv, err := s.repo.Create(ctx, in.SessionID, in.PackageID, patientID, total, nil)
	if err != nil {
		return nil, err
	}

	clinicName, err := s.placeholderRepo.Get(ctx, PlaceholderClinicName)
	if err != nil {
		return nil, err
	}
	clinicAddress, err := s.placeholderRepo.Get(ctx, PlaceholderClinicAddress)
	if err != nil {
		return nil, err
	}
	footerText, err := s.placeholderRepo.Get(ctx, PlaceholderFooterText)
	if err != nil {
		return nil, err
	}

	pdfBytes, err := renderPDF(pdfData{
		ClinicName:    clinicName,
		ClinicAddress: clinicAddress,
		FooterText:    footerText,
		InvoiceID:     inv.ID,
		PatientName:   pat.FullName,
		LineItem:      lineItem,
		Total:         total,
		IssuedAt:      inv.IssuedAt,
	})
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(s.storageDir, 0o755); err != nil {
		return nil, err
	}
	pdfPath := filepath.Join(s.storageDir, inv.ID+".pdf")
	if err := os.WriteFile(pdfPath, pdfBytes, 0o644); err != nil {
		return nil, err
	}

	return s.repo.SetPDFPath(ctx, inv.ID, pdfPath)
}

func (s *Service) Get(ctx context.Context, id string) (*Invoice, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) List(ctx context.Context) ([]*Invoice, error) {
	return s.repo.List(ctx)
}

func (s *Service) SetPlaceholder(ctx context.Context, key, value string) (*TemplatePlaceholder, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, ErrPlaceholderKeyRequired
	}
	return s.placeholderRepo.Upsert(ctx, key, value)
}

func (s *Service) ListPlaceholders(ctx context.Context) ([]*TemplatePlaceholder, error) {
	return s.placeholderRepo.List(ctx)
}
