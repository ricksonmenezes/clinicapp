package service

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, name string, description *string, price float64, requiresConsultant, active bool) (*Service, error) {
	s := &Service{}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO services (name, description, price, requires_consultant, active)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, name, description, price, requires_consultant, active, created_at, updated_at
	`, name, description, price, requiresConsultant, active).Scan(
		&s.ID, &s.Name, &s.Description, &s.Price, &s.RequiresConsultant, &s.Active, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (r *Repository) GetByID(ctx context.Context, id string) (*Service, error) {
	s := &Service{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, description, price, requires_consultant, active, created_at, updated_at
		FROM services WHERE id = $1
	`, id).Scan(&s.ID, &s.Name, &s.Description, &s.Price, &s.RequiresConsultant, &s.Active, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return s, nil
}

func (r *Repository) List(ctx context.Context) ([]*Service, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, description, price, requires_consultant, active, created_at, updated_at
		FROM services ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var services []*Service
	for rows.Next() {
		s := &Service{}
		if err := rows.Scan(&s.ID, &s.Name, &s.Description, &s.Price, &s.RequiresConsultant, &s.Active, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		services = append(services, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return services, nil
}

func (r *Repository) Update(ctx context.Context, id, name string, description *string, price float64, requiresConsultant, active bool) (*Service, error) {
	s := &Service{}
	err := r.pool.QueryRow(ctx, `
		UPDATE services
		SET name = $1, description = $2, price = $3, requires_consultant = $4, active = $5, updated_at = now()
		WHERE id = $6
		RETURNING id, name, description, price, requires_consultant, active, created_at, updated_at
	`, name, description, price, requiresConsultant, active, id).Scan(
		&s.ID, &s.Name, &s.Description, &s.Price, &s.RequiresConsultant, &s.Active, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return s, nil
}
