package consultant

import (
	"context"
	"errors"
	"strings"

	"clinicapp/backend/internal/auth"
)

type Service struct {
	repo     *Repository
	userRepo *auth.Repository
}

func NewService(repo *Repository, userRepo *auth.Repository) *Service {
	return &Service{repo: repo, userRepo: userRepo}
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
