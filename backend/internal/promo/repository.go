package promo

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PackageRepository struct {
	pool *pgxpool.Pool
}

func NewPackageRepository(pool *pgxpool.Pool) *PackageRepository {
	return &PackageRepository{pool: pool}
}

func (r *PackageRepository) Create(ctx context.Context, serviceID, name string, sessionCount int, price float64, active bool) (*Package, error) {
	p := &Package{}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO packages (service_id, name, session_count, price, active)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, service_id, name, session_count, price, active, created_at, updated_at
	`, serviceID, name, sessionCount, price, active).Scan(
		&p.ID, &p.ServiceID, &p.Name, &p.SessionCount, &p.Price, &p.Active, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (r *PackageRepository) GetByID(ctx context.Context, id string) (*Package, error) {
	p := &Package{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, service_id, name, session_count, price, active, created_at, updated_at
		FROM packages WHERE id = $1
	`, id).Scan(&p.ID, &p.ServiceID, &p.Name, &p.SessionCount, &p.Price, &p.Active, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPackageNotFound
		}
		return nil, err
	}
	return p, nil
}

func (r *PackageRepository) List(ctx context.Context) ([]*Package, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, service_id, name, session_count, price, active, created_at, updated_at
		FROM packages ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var packages []*Package
	for rows.Next() {
		p := &Package{}
		if err := rows.Scan(&p.ID, &p.ServiceID, &p.Name, &p.SessionCount, &p.Price, &p.Active, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		packages = append(packages, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return packages, nil
}

func (r *PackageRepository) Update(ctx context.Context, id, name string, sessionCount int, price float64, active bool) (*Package, error) {
	p := &Package{}
	err := r.pool.QueryRow(ctx, `
		UPDATE packages SET name = $1, session_count = $2, price = $3, active = $4, updated_at = now()
		WHERE id = $5
		RETURNING id, service_id, name, session_count, price, active, created_at, updated_at
	`, name, sessionCount, price, active, id).Scan(
		&p.ID, &p.ServiceID, &p.Name, &p.SessionCount, &p.Price, &p.Active, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPackageNotFound
		}
		return nil, err
	}
	return p, nil
}

type PatientPackageRepository struct {
	pool *pgxpool.Pool
}

func NewPatientPackageRepository(pool *pgxpool.Pool) *PatientPackageRepository {
	return &PatientPackageRepository{pool: pool}
}

func (r *PatientPackageRepository) Create(ctx context.Context, patientID, packageID string, principalConsultant *string, sessionsRemaining int) (*PatientPackage, error) {
	pp := &PatientPackage{}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO patient_packages (patient_id, package_id, principal_consultant, sessions_remaining)
		VALUES ($1, $2, $3, $4)
		RETURNING id, patient_id, package_id, principal_consultant, sessions_remaining, purchased_at, created_at, updated_at
	`, patientID, packageID, principalConsultant, sessionsRemaining).Scan(
		&pp.ID, &pp.PatientID, &pp.PackageID, &pp.PrincipalConsultant, &pp.SessionsRemaining, &pp.PurchasedAt, &pp.CreatedAt, &pp.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return pp, nil
}

func (r *PatientPackageRepository) GetByID(ctx context.Context, id string) (*PatientPackage, error) {
	pp := &PatientPackage{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, patient_id, package_id, principal_consultant, sessions_remaining, purchased_at, created_at, updated_at
		FROM patient_packages WHERE id = $1
	`, id).Scan(&pp.ID, &pp.PatientID, &pp.PackageID, &pp.PrincipalConsultant, &pp.SessionsRemaining, &pp.PurchasedAt, &pp.CreatedAt, &pp.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPatientPackageNotFound
		}
		return nil, err
	}
	return pp, nil
}

func (r *PatientPackageRepository) List(ctx context.Context) ([]*PatientPackage, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, patient_id, package_id, principal_consultant, sessions_remaining, purchased_at, created_at, updated_at
		FROM patient_packages ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var patientPackages []*PatientPackage
	for rows.Next() {
		pp := &PatientPackage{}
		if err := rows.Scan(&pp.ID, &pp.PatientID, &pp.PackageID, &pp.PrincipalConsultant, &pp.SessionsRemaining, &pp.PurchasedAt, &pp.CreatedAt, &pp.UpdatedAt); err != nil {
			return nil, err
		}
		patientPackages = append(patientPackages, pp)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return patientPackages, nil
}
