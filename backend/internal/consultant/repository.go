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

// GetByUserID looks up a consultant profile by the underlying auth user id —
// used to resolve "the consultant profile of whoever is calling this
// endpoint" (e.g. Rx authorship) rather than trusting a caller-supplied
// consultant_id.
func (r *Repository) GetByUserID(ctx context.Context, userID string) (*Consultant, error) {
	c := &Consultant{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, full_name, default_commission, created_at, updated_at
		FROM consultants WHERE user_id = $1
	`, userID).Scan(&c.ID, &c.UserID, &c.FullName, &c.DefaultCommission, &c.CreatedAt, &c.UpdatedAt)
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

// UpsertServiceCommission sets (creating or replacing) the override rate for
// a consultant/service pair. It's an upsert rather than separate create/update
// endpoints because the override is conceptually a single value the admin is
// setting, not a record with its own lifecycle.
func (r *Repository) UpsertServiceCommission(ctx context.Context, consultantID, serviceID string, commission float64) (*ServiceCommission, error) {
	sc := &ServiceCommission{}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO consultant_service_commission (consultant_id, service_id, commission)
		VALUES ($1, $2, $3)
		ON CONFLICT (consultant_id, service_id) DO UPDATE SET commission = $3, updated_at = now()
		RETURNING id, consultant_id, service_id, commission, created_at, updated_at
	`, consultantID, serviceID, commission).Scan(
		&sc.ID, &sc.ConsultantID, &sc.ServiceID, &sc.Commission, &sc.CreatedAt, &sc.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return sc, nil
}

// GetServiceCommission looks up the override rate for a single
// consultant/service pair — used by internal/session to resolve the
// service-level step of the commission resolution order.
func (r *Repository) GetServiceCommission(ctx context.Context, consultantID, serviceID string) (*ServiceCommission, error) {
	sc := &ServiceCommission{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, consultant_id, service_id, commission, created_at, updated_at
		FROM consultant_service_commission WHERE consultant_id = $1 AND service_id = $2
	`, consultantID, serviceID).Scan(&sc.ID, &sc.ConsultantID, &sc.ServiceID, &sc.Commission, &sc.CreatedAt, &sc.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrServiceCommissionNotFound
		}
		return nil, err
	}
	return sc, nil
}

func (r *Repository) ListServiceCommissions(ctx context.Context, consultantID string) ([]*ServiceCommission, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, consultant_id, service_id, commission, created_at, updated_at
		FROM consultant_service_commission WHERE consultant_id = $1 ORDER BY created_at DESC
	`, consultantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var overrides []*ServiceCommission
	for rows.Next() {
		sc := &ServiceCommission{}
		if err := rows.Scan(&sc.ID, &sc.ConsultantID, &sc.ServiceID, &sc.Commission, &sc.CreatedAt, &sc.UpdatedAt); err != nil {
			return nil, err
		}
		overrides = append(overrides, sc)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return overrides, nil
}
