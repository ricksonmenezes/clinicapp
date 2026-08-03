package patient

import (
	"context"
	"errors"
	"strings"
	"time"

	"clinicapp/backend/internal/auth"
)

type Service struct {
	repo     *Repository
	userRepo *auth.Repository
}

func NewService(repo *Repository, userRepo *auth.Repository) *Service {
	return &Service{repo: repo, userRepo: userRepo}
}

func (s *Service) Create(ctx context.Context, userID, fullName string, dob *time.Time, phone, notes *string) (*Patient, error) {
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
	if user.Role != auth.RolePatient {
		return nil, ErrInvalidRole
	}

	return s.repo.Create(ctx, userID, fullName, dob, phone, notes)
}

func (s *Service) Get(ctx context.Context, id string) (*Patient, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) List(ctx context.Context) ([]*Patient, error) {
	return s.repo.List(ctx)
}

func (s *Service) Update(ctx context.Context, id, fullName string, dob *time.Time, phone, notes *string) (*Patient, error) {
	fullName = strings.TrimSpace(fullName)
	if fullName == "" {
		return nil, ErrValidation
	}
	return s.repo.Update(ctx, id, fullName, dob, phone, notes)
}
