package consultant

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const uniqueViolation = "23505"

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, userID, fullName string, defaultCommission float64) (*Consultant, error) {
	c := &Consultant{}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO consultants (user_id, full_name, default_commission)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, full_name, default_commission, created_at, updated_at
	`, userID, fullName, defaultCommission).Scan(
		&c.ID, &c.UserID, &c.FullName, &c.DefaultCommission, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			return nil, ErrAlreadyExists
		}
		return nil, err
	}
	return c, nil
}

func (r *Repository) GetByID(ctx context.Context, id string) (*Consultant, error) {
	c := &Consultant{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, full_name, default_commission, created_at, updated_at
		FROM consultants WHERE id = $1
	`, id).Scan(&c.ID, &c.UserID, &c.FullName, &c.DefaultCommission, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return c, nil
}

func (r *Repository) List(ctx context.Context) ([]*Consultant, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, full_name, default_commission, created_at, updated_at
		FROM consultants ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var consultants []*Consultant
	for rows.Next() {
		c := &Consultant{}
		if err := rows.Scan(&c.ID, &c.UserID, &c.FullName, &c.DefaultCommission, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		consultants = append(consultants, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return consultants, nil
}

func (r *Repository) Update(ctx context.Context, id, fullName string, defaultCommission float64) (*Consultant, error) {
	c := &Consultant{}
	err := r.pool.QueryRow(ctx, `
		UPDATE consultants SET full_name = $1, default_commission = $2, updated_at = now()
		WHERE id = $3
		RETURNING id, user_id, full_name, default_commission, created_at, updated_at
	`, fullName, defaultCommission, id).Scan(
		&c.ID, &c.UserID, &c.FullName, &c.DefaultCommission, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return c, nil
}
