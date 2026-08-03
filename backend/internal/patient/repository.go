package patient

import (
	"context"
	"errors"
	"time"

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

func (r *Repository) Create(ctx context.Context, userID, fullName string, dob *time.Time, phone, notes *string) (*Patient, error) {
	p := &Patient{}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO patients (user_id, full_name, dob, phone, notes)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, user_id, full_name, dob, phone, notes, created_at, updated_at
	`, userID, fullName, dob, phone, notes).Scan(
		&p.ID, &p.UserID, &p.FullName, &p.DOB, &p.Phone, &p.Notes, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			return nil, ErrAlreadyExists
		}
		return nil, err
	}
	return p, nil
}

func (r *Repository) GetByID(ctx context.Context, id string) (*Patient, error) {
	p := &Patient{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, full_name, dob, phone, notes, created_at, updated_at
		FROM patients WHERE id = $1
	`, id).Scan(&p.ID, &p.UserID, &p.FullName, &p.DOB, &p.Phone, &p.Notes, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return p, nil
}

// GetByUserID looks up a patient profile by the underlying auth user id —
// used to resolve "the patient profile of whoever is calling this endpoint"
// (self-service portal routes) rather than trusting a caller-supplied
// patient_id, same pattern as consultant.Repository.GetByUserID.
func (r *Repository) GetByUserID(ctx context.Context, userID string) (*Patient, error) {
	p := &Patient{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, full_name, dob, phone, notes, created_at, updated_at
		FROM patients WHERE user_id = $1
	`, userID).Scan(&p.ID, &p.UserID, &p.FullName, &p.DOB, &p.Phone, &p.Notes, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return p, nil
}

func (r *Repository) List(ctx context.Context) ([]*Patient, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, full_name, dob, phone, notes, created_at, updated_at
		FROM patients ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var patients []*Patient
	for rows.Next() {
		p := &Patient{}
		if err := rows.Scan(&p.ID, &p.UserID, &p.FullName, &p.DOB, &p.Phone, &p.Notes, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		patients = append(patients, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return patients, nil
}

func (r *Repository) Update(ctx context.Context, id, fullName string, dob *time.Time, phone, notes *string) (*Patient, error) {
	p := &Patient{}
	err := r.pool.QueryRow(ctx, `
		UPDATE patients SET full_name = $1, dob = $2, phone = $3, notes = $4, updated_at = now()
		WHERE id = $5
		RETURNING id, user_id, full_name, dob, phone, notes, created_at, updated_at
	`, fullName, dob, phone, notes, id).Scan(
		&p.ID, &p.UserID, &p.FullName, &p.DOB, &p.Phone, &p.Notes, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return p, nil
}
