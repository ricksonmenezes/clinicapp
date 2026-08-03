package invoice

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

func (r *Repository) Create(ctx context.Context, sessionID, packageID *string, patientID string, total float64, pdfPath *string) (*Invoice, error) {
	inv := &Invoice{}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO invoices (session_id, package_id, patient_id, total, pdf_path)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, session_id, package_id, patient_id, total, issued_at, pdf_path
	`, sessionID, packageID, patientID, total, pdfPath).Scan(
		&inv.ID, &inv.SessionID, &inv.PackageID, &inv.PatientID, &inv.Total, &inv.IssuedAt, &inv.PDFPath,
	)
	if err != nil {
		return nil, err
	}
	return inv, nil
}

func (r *Repository) GetByID(ctx context.Context, id string) (*Invoice, error) {
	inv := &Invoice{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, session_id, package_id, patient_id, total, issued_at, pdf_path
		FROM invoices WHERE id = $1
	`, id).Scan(&inv.ID, &inv.SessionID, &inv.PackageID, &inv.PatientID, &inv.Total, &inv.IssuedAt, &inv.PDFPath)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return inv, nil
}

func (r *Repository) List(ctx context.Context) ([]*Invoice, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, session_id, package_id, patient_id, total, issued_at, pdf_path
		FROM invoices ORDER BY issued_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var invoices []*Invoice
	for rows.Next() {
		inv := &Invoice{}
		if err := rows.Scan(&inv.ID, &inv.SessionID, &inv.PackageID, &inv.PatientID, &inv.Total, &inv.IssuedAt, &inv.PDFPath); err != nil {
			return nil, err
		}
		invoices = append(invoices, inv)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return invoices, nil
}

// SetPDFPath records where the generated PDF for an invoice was written.
// Called once, right after Create — invoices are created with pdf_path NULL
// because the on-disk filename is derived from the DB-assigned id.
func (r *Repository) SetPDFPath(ctx context.Context, id, pdfPath string) (*Invoice, error) {
	inv := &Invoice{}
	err := r.pool.QueryRow(ctx, `
		UPDATE invoices SET pdf_path = $1 WHERE id = $2
		RETURNING id, session_id, package_id, patient_id, total, issued_at, pdf_path
	`, pdfPath, id).Scan(&inv.ID, &inv.SessionID, &inv.PackageID, &inv.PatientID, &inv.Total, &inv.IssuedAt, &inv.PDFPath)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return inv, nil
}

type PlaceholderRepository struct {
	pool *pgxpool.Pool
}

func NewPlaceholderRepository(pool *pgxpool.Pool) *PlaceholderRepository {
	return &PlaceholderRepository{pool: pool}
}

func (r *PlaceholderRepository) Upsert(ctx context.Context, key, value string) (*TemplatePlaceholder, error) {
	p := &TemplatePlaceholder{}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO invoice_template_placeholders (key, value)
		VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = now()
		RETURNING id, key, value, updated_at
	`, key, value).Scan(&p.ID, &p.Key, &p.Value, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (r *PlaceholderRepository) List(ctx context.Context) ([]*TemplatePlaceholder, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, key, value, updated_at FROM invoice_template_placeholders ORDER BY key
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var placeholders []*TemplatePlaceholder
	for rows.Next() {
		p := &TemplatePlaceholder{}
		if err := rows.Scan(&p.ID, &p.Key, &p.Value, &p.UpdatedAt); err != nil {
			return nil, err
		}
		placeholders = append(placeholders, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return placeholders, nil
}

// Get returns the placeholder's value, or "" if the key has never been set —
// PDF rendering treats an unset placeholder as blank rather than erroring,
// so admins can generate invoices before filling in every field.
func (r *PlaceholderRepository) Get(ctx context.Context, key string) (string, error) {
	var value string
	err := r.pool.QueryRow(ctx, `SELECT value FROM invoice_template_placeholders WHERE key = $1`, key).Scan(&value)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return value, nil
}
