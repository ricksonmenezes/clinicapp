package consultant

import (
	"context"
	"errors"
	"strings"

	"clinicapp/backend/internal/auth"
	"clinicapp/backend/internal/service"
)

type Service struct {
	repo        *Repository
	userRepo    *auth.Repository
	serviceRepo *service.Repository
}

func NewService(repo *Repository, userRepo *auth.Repository, serviceRepo *service.Repository) *Service {
	return &Service{repo: repo, userRepo: userRepo, serviceRepo: serviceRepo}
}

func (s *Service) Create(ctx context.Context, userID, fullName string, defaultCommission float64) (*Consultant, error) {
	fullName = strings.TrimSpace(fullName)
	if fullName == "" {
		return nil, ErrValidation
	}
	if defaultCommission < 0 || defaultCommission > 100 {
		return nil, ErrInvalidCommission
	}

	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	if user.Role != auth.RoleClinician {
		return nil, ErrInvalidRole
	}

	return s.repo.Create(ctx, userID, fullName, defaultCommission)
}

func (s *Service) Get(ctx context.Context, id string) (*Consultant, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) List(ctx context.Context) ([]*Consultant, error) {
	return s.repo.List(ctx)
}

func (s *Service) Update(ctx context.Context, id, fullName string, defaultCommission float64) (*Consultant, error) {
	fullName = strings.TrimSpace(fullName)
	if fullName == "" {
		return nil, ErrValidation
	}
	if defaultCommission < 0 || defaultCommission > 100 {
		return nil, ErrInvalidCommission
	}
	return s.repo.Update(ctx, id, fullName, defaultCommission)
}

func (s *Service) SetServiceCommission(ctx context.Context, consultantID, serviceID string, commission float64) (*ServiceCommission, error) {
	if commission < 0 || commission > 100 {
		return nil, ErrInvalidCommission
	}
	if _, err := s.repo.GetByID(ctx, consultantID); err != nil {
		return nil, err
	}
	if _, err := s.serviceRepo.GetByID(ctx, serviceID); err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return nil, ErrServiceNotFound
		}
		return nil, err
	}
	return s.repo.UpsertServiceCommission(ctx, consultantID, serviceID, commission)
}

func (s *Service) ListServiceCommissions(ctx context.Context, consultantID string) ([]*ServiceCommission, error) {
	if _, err := s.repo.GetByID(ctx, consultantID); err != nil {
		return nil, err
	}
	return s.repo.ListServiceCommissions(ctx, consultantID)
}
