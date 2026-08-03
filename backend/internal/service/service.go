package service

import (
	"context"
	"strings"
)

// Manager is this package's business-logic layer. It's named Manager rather
// than the Service/NewService convention used by sibling packages (patient,
// consultant, attendant) to avoid colliding with the Service domain type,
// which is named after this package's own entity.
type Manager struct {
	repo *Repository
}

func NewManager(repo *Repository) *Manager {
	return &Manager{repo: repo}
}

func (m *Manager) Create(ctx context.Context, name string, description *string, price float64, requiresConsultant, active bool) (*Service, error) {
	name = strings.TrimSpace(name)
	if name == "" || price < 0 {
		return nil, ErrValidation
	}
	return m.repo.Create(ctx, name, description, price, requiresConsultant, active)
}

func (m *Manager) Get(ctx context.Context, id string) (*Service, error) {
	return m.repo.GetByID(ctx, id)
}

func (m *Manager) List(ctx context.Context) ([]*Service, error) {
	return m.repo.List(ctx)
}

func (m *Manager) Update(ctx context.Context, id, name string, description *string, price float64, requiresConsultant, active bool) (*Service, error) {
	name = strings.TrimSpace(name)
	if name == "" || price < 0 {
		return nil, ErrValidation
	}
	return m.repo.Update(ctx, id, name, description, price, requiresConsultant, active)
}
