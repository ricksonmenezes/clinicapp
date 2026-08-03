package prescription

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"clinicapp/backend/internal/consultant"
	"clinicapp/backend/internal/invoice"
	"clinicapp/backend/internal/patient"
	"clinicapp/backend/internal/session"
)

type Service struct {
	repo            *Repository
	consultantRepo  *consultant.Repository
	patientRepo     *patient.Repository
	sessionRepo     *session.Repository
	placeholderRepo *invoice.PlaceholderRepository
	storageDir      string
}

func NewService(
	repo *Repository,
	consultantRepo *consultant.Repository,
	patientRepo *patient.Repository,
	sessionRepo *session.Repository,
	placeholderRepo *invoice.PlaceholderRepository,
	storageDir string,
) *Service {
	return &Service{
		repo:            repo,
		consultantRepo:  consultantRepo,
		patientRepo:     patientRepo,
		sessionRepo:     sessionRepo,
		placeholderRepo: placeholderRepo,
		storageDir:      storageDir,
	}
}

type CreateInput struct {
	PatientID string
	SessionID *string
	Content   string
}

// Create authors an Rx as the consultant profile belonging to callerUserID
// (the authenticated JWT subject) — never a caller-supplied consultant_id,
// so a clinician can only issue prescriptions under their own name.
func (s *Service) Create(ctx context.Context, callerUserID string, in CreateInput) (*Prescription, error) {
	content := strings.TrimSpace(in.Content)
	if in.PatientID == "" || content == "" {
		return nil, ErrValidation
	}

	cons, err := s.consultantRepo.GetByUserID(ctx, callerUserID)
	if err != nil {
		if errors.Is(err, consultant.ErrNotFound) {
			return nil, ErrConsultantProfileMissing
		}
		return nil, err
	}

	pat, err := s.patientRepo.GetByID(ctx, in.PatientID)
	if err != nil {
		if errors.Is(err, patient.ErrNotFound) {
			return nil, ErrPatientNotFound
		}
		return nil, err
	}

	if in.SessionID != nil {
		sess, err := s.sessionRepo.GetByID(ctx, *in.SessionID)
		if err != nil {
			if errors.Is(err, session.ErrNotFound) {
				return nil, ErrSessionNotFound
			}
			return nil, err
		}
		if sess.PatientID != in.PatientID {
			return nil, ErrSessionPatientMismatch
		}
	}

	rx, err := s.repo.Create(ctx, in.SessionID, cons.ID, in.PatientID, content, nil)
	if err != nil {
		return nil, err
	}

	// Reuse the clinic identity placeholders admins already maintain for
	// invoices (clinic_name/clinic_address/footer_text) rather than adding a
	// second admin-editable letterhead config — it's the same clinic
	// identity, not invoice-specific data, and CLAUDE.md's "never hardcode
	// clinic name/address/footer" applies to every clinic-generated PDF.
	clinicName, err := s.placeholderRepo.Get(ctx, invoice.PlaceholderClinicName)
	if err != nil {
		return nil, err
	}
	clinicAddress, err := s.placeholderRepo.Get(ctx, invoice.PlaceholderClinicAddress)
	if err != nil {
		return nil, err
	}
	footerText, err := s.placeholderRepo.Get(ctx, invoice.PlaceholderFooterText)
	if err != nil {
		return nil, err
	}

	pdfBytes, err := renderPDF(pdfData{
		ClinicName:     clinicName,
		ClinicAddress:  clinicAddress,
		FooterText:     footerText,
		PrescriptionID: rx.ID,
		PatientName:    pat.FullName,
		ConsultantName: cons.FullName,
		Content:        content,
		IssuedAt:       rx.IssuedAt,
	})
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(s.storageDir, 0o755); err != nil {
		return nil, err
	}
	pdfPath := filepath.Join(s.storageDir, rx.ID+".pdf")
	if err := os.WriteFile(pdfPath, pdfBytes, 0o644); err != nil {
		return nil, err
	}

	return s.repo.SetPDFPath(ctx, rx.ID, pdfPath)
}

func (s *Service) Get(ctx context.Context, id string) (*Prescription, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) List(ctx context.Context, filter ListFilter) ([]*Prescription, error) {
	return s.repo.List(ctx, filter)
}
