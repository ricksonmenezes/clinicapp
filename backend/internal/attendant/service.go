package attendant

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

func (s *Service) Create(ctx context.Context, userID, fullName string) (*Attendant, error) {
	fullName = strings.TrimSpace(fullName)
	if fullName == "" {
		return nil, ErrValidation
	}

	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	if user.Role != auth.RoleAttendant {
		return nil, ErrInvalidRole
	}

	return s.repo.Create(ctx, userID, fullName)
}

func (s *Service) Get(ctx context.Context, id string) (*Attendant, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) List(ctx context.Context) ([]*Attendant, error) {
	return s.repo.List(ctx)
}

func (s *Service) Update(ctx context.Context, id, fullName string) (*Attendant, error) {
	fullName = strings.TrimSpace(fullName)
	if fullName == "" {
		return nil, ErrValidation
	}
	return s.repo.Update(ctx, id, fullName)
}
