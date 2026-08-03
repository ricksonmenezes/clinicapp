package attendant

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

func (r *Repository) Create(ctx context.Context, userID, fullName string) (*Attendant, error) {
	a := &Attendant{}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO attendants (user_id, full_name)
		VALUES ($1, $2)
		RETURNING id, user_id, full_name, created_at, updated_at
	`, userID, fullName).Scan(&a.ID, &a.UserID, &a.FullName, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			return nil, ErrAlreadyExists
		}
		return nil, err
	}
	return a, nil
}

func (r *Repository) GetByID(ctx context.Context, id string) (*Attendant, error) {
	a := &Attendant{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, full_name, created_at, updated_at
		FROM attendants WHERE id = $1
	`, id).Scan(&a.ID, &a.UserID, &a.FullName, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return a, nil
}

func (r *Repository) List(ctx context.Context) ([]*Attendant, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, full_name, created_at, updated_at
		FROM attendants ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attendants []*Attendant
	for rows.Next() {
		a := &Attendant{}
		if err := rows.Scan(&a.ID, &a.UserID, &a.FullName, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		attendants = append(attendants, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return attendants, nil
}

func (r *Repository) Update(ctx context.Context, id, fullName string) (*Attendant, error) {
	a := &Attendant{}
	err := r.pool.QueryRow(ctx, `
		UPDATE attendants SET full_name = $1, updated_at = now()
		WHERE id = $2
		RETURNING id, user_id, full_name, created_at, updated_at
	`, fullName, id).Scan(&a.ID, &a.UserID, &a.FullName, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return a, nil
}
