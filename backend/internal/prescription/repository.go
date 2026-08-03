package prescription

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, sessionID *string, consultantID, patientID, content string, pdfPath *string) (*Prescription, error) {
	p := &Prescription{}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO prescriptions (session_id, consultant_id, patient_id, content, pdf_path)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, session_id, consultant_id, patient_id, content, issued_at, pdf_path
	`, sessionID, consultantID, patientID, content, pdfPath).Scan(
		&p.ID, &p.SessionID, &p.ConsultantID, &p.PatientID, &p.Content, &p.IssuedAt, &p.PDFPath,
	)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (r *Repository) GetByID(ctx context.Context, id string) (*Prescription, error) {
	p := &Prescription{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, session_id, consultant_id, patient_id, content, issued_at, pdf_path
		FROM prescriptions WHERE id = $1
	`, id).Scan(&p.ID, &p.SessionID, &p.ConsultantID, &p.PatientID, &p.Content, &p.IssuedAt, &p.PDFPath)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return p, nil
}

type ListFilter struct {
	PatientID    string
	ConsultantID string
}

// List returns prescriptions matching the filter, most recent first —
// filtering by PatientID or ConsultantID serves per-patient/per-consultant
// Rx history without dedicated endpoints, matching session.Repository.List.
func (r *Repository) List(ctx context.Context, filter ListFilter) ([]*Prescription, error) {
	query := `
		SELECT id, session_id, consultant_id, patient_id, content, issued_at, pdf_path
		FROM prescriptions
	`
	var conditions []string
	var args []any
	if filter.PatientID != "" {
		args = append(args, filter.PatientID)
		conditions = append(conditions, fmt.Sprintf("patient_id = $%d", len(args)))
	}
	if filter.ConsultantID != "" {
		args = append(args, filter.ConsultantID)
		conditions = append(conditions, fmt.Sprintf("consultant_id = $%d", len(args)))
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY issued_at DESC"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prescriptions []*Prescription
	for rows.Next() {
		p := &Prescription{}
		if err := rows.Scan(&p.ID, &p.SessionID, &p.ConsultantID, &p.PatientID, &p.Content, &p.IssuedAt, &p.PDFPath); err != nil {
			return nil, err
		}
		prescriptions = append(prescriptions, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return prescriptions, nil
}

// SetPDFPath records where the generated PDF for a prescription was
// written. Called once, right after Create — the on-disk filename is
// derived from the DB-assigned id, so pdf_path starts NULL.
func (r *Repository) SetPDFPath(ctx context.Context, id, pdfPath string) (*Prescription, error) {
	p := &Prescription{}
	err := r.pool.QueryRow(ctx, `
		UPDATE prescriptions SET pdf_path = $1 WHERE id = $2
		RETURNING id, session_id, consultant_id, patient_id, content, issued_at, pdf_path
	`, pdfPath, id).Scan(&p.ID, &p.SessionID, &p.ConsultantID, &p.PatientID, &p.Content, &p.IssuedAt, &p.PDFPath)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return p, nil
}
