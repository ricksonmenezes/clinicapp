package promo

import (
	"context"
	"errors"
	"strings"

	"clinicapp/backend/internal/consultant"
	"clinicapp/backend/internal/patient"
	"clinicapp/backend/internal/service"
)

// PackageManager and PatientPackageManager are named Manager, not Service,
// to avoid confusion with the clinic Service entity (internal/service) that
// packages are built on top of.
type PackageManager struct {
	repo        *PackageRepository
	serviceRepo *service.Repository
}

func NewPackageManager(repo *PackageRepository, serviceRepo *service.Repository) *PackageManager {
	return &PackageManager{repo: repo, serviceRepo: serviceRepo}
}

func (m *PackageManager) Create(ctx context.Context, serviceID, name string, sessionCount int, price float64, active bool) (*Package, error) {
	name = strings.TrimSpace(name)
	if name == "" || sessionCount <= 0 || price < 0 {
		return nil, ErrValidation
	}
	if _, err := m.serviceRepo.GetByID(ctx, serviceID); err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return nil, ErrServiceNotFound
		}
		return nil, err
	}
	return m.repo.Create(ctx, serviceID, name, sessionCount, price, active)
}

func (m *PackageManager) Get(ctx context.Context, id string) (*Package, error) {
	return m.repo.GetByID(ctx, id)
}

func (m *PackageManager) List(ctx context.Context) ([]*Package, error) {
	return m.repo.List(ctx)
}

func (m *PackageManager) Update(ctx context.Context, id, name string, sessionCount int, price float64, active bool) (*Package, error) {
	name = strings.TrimSpace(name)
	if name == "" || sessionCount <= 0 || price < 0 {
		return nil, ErrValidation
	}
	return m.repo.Update(ctx, id, name, sessionCount, price, active)
}

type PatientPackageManager struct {
	repo           *PatientPackageRepository
	packageRepo    *PackageRepository
	patientRepo    *patient.Repository
	consultantRepo *consultant.Repository
}

func NewPatientPackageManager(repo *PatientPackageRepository, packageRepo *PackageRepository, patientRepo *patient.Repository, consultantRepo *consultant.Repository) *PatientPackageManager {
	return &PatientPackageManager{repo: repo, packageRepo: packageRepo, patientRepo: patientRepo, consultantRepo: consultantRepo}
}

// Subscribe enrolls a patient in a package: sessions_remaining is seeded from
// the package's session_count at the moment of purchase, so later edits to
// the package definition don't retroactively change an existing subscription.
func (m *PatientPackageManager) Subscribe(ctx context.Context, patientID, packageID string, principalConsultant *string) (*PatientPackage, error) {
	if _, err := m.patientRepo.GetByID(ctx, patientID); err != nil {
		if errors.Is(err, patient.ErrNotFound) {
			return nil, ErrPatientNotFound
		}
		return nil, err
	}

	pkg, err := m.packageRepo.GetByID(ctx, packageID)
	if err != nil {
		if errors.Is(err, ErrPackageNotFound) {
			return nil, ErrInvalidPackageID
		}
		return nil, err
	}

	if principalConsultant != nil {
		if _, err := m.consultantRepo.GetByID(ctx, *principalConsultant); err != nil {
			if errors.Is(err, consultant.ErrNotFound) {
				return nil, ErrConsultantNotFound
			}
			return nil, err
		}
	}

	return m.repo.Create(ctx, patientID, packageID, principalConsultant, pkg.SessionCount)
}

func (m *PatientPackageManager) Get(ctx context.Context, id string) (*PatientPackage, error) {
	return m.repo.GetByID(ctx, id)
}

func (m *PatientPackageManager) List(ctx context.Context) ([]*PatientPackage, error) {
	return m.repo.List(ctx)
}
