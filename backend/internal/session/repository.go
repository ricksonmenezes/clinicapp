package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

type CreateParams struct {
	PatientID        string
	ServiceID        string
	PatientPackageID *string
	ScheduledAt      time.Time
	CompletedAt      *time.Time
	ConsultantID     *string
	Notes            *string
	AttendantIDs     []string

	// Snapshot is nil when ConsultantID is nil — an attendant-only service
	// has no commission to record.
	Snapshot *CommissionSnapshot
}

// Create inserts a session and everything that must land atomically with it:
// the attendant roster, the commission snapshot (if a consultant performed
// it), and — if this session is drawn from a package — the decrement of
// that package's sessions_remaining. All in one transaction so a mid-write
// failure can't leave sessions_remaining out of sync with actual usage.
func (r *Repository) Create(ctx context.Context, p CreateParams) (*Session, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	s := &Session{}
	err = tx.QueryRow(ctx, `
		INSERT INTO sessions (patient_id, service_id, patient_package_id, scheduled_at, completed_at, consultant_id, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, patient_id, service_id, patient_package_id, scheduled_at, completed_at, consultant_id, notes, created_at
	`, p.PatientID, p.ServiceID, p.PatientPackageID, p.ScheduledAt, p.CompletedAt, p.ConsultantID, p.Notes).Scan(
		&s.ID, &s.PatientID, &s.ServiceID, &s.PatientPackageID, &s.ScheduledAt, &s.CompletedAt, &s.ConsultantID, &s.Notes, &s.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	for _, attendantID := range p.AttendantIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO session_attendants (session_id, attendant_id) VALUES ($1, $2)
		`, s.ID, attendantID); err != nil {
			return nil, err
		}
	}
	s.AttendantIDs = p.AttendantIDs

	if p.Snapshot != nil {
		snap := *p.Snapshot
		err = tx.QueryRow(ctx, `
			INSERT INTO session_commission_snapshot (session_id, consultant_id, commission_rate, resolution_source, clinic_amount, consultant_amount)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING id, session_id, consultant_id, commission_rate, resolution_source, clinic_amount, consultant_amount, created_at
		`, s.ID, snap.ConsultantID, snap.CommissionRate, snap.ResolutionSource, snap.ClinicAmount, snap.ConsultantAmount).Scan(
			&snap.ID, &snap.SessionID, &snap.ConsultantID, &snap.CommissionRate, &snap.ResolutionSource, &snap.ClinicAmount, &snap.ConsultantAmount, &snap.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		s.CommissionSnapshot = &snap
	}

	if p.PatientPackageID != nil {
		// patient_packages is owned by internal/promo, but the decrement
		// must commit atomically with the session insert above and this
		// codebase has no shared cross-package transaction/querier
		// abstraction — so it's done here directly rather than introducing
		// one for a single call site. The `sessions_remaining > 0` guard
		// makes this the actual concurrency-safe check; the service layer's
		// pre-check is just a fast path for a friendlier error on the
		// common case.
		tag, err := tx.Exec(ctx, `
			UPDATE patient_packages SET sessions_remaining = sessions_remaining - 1, updated_at = now()
			WHERE id = $1 AND sessions_remaining > 0
		`, *p.PatientPackageID)
		if err != nil {
			return nil, err
		}
		if tag.RowsAffected() == 0 {
			return nil, ErrNoSessionsRemaining
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return s, nil
}

func (r *Repository) GetByID(ctx context.Context, id string) (*Session, error) {
	s := &Session{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, patient_id, service_id, patient_package_id, scheduled_at, completed_at, consultant_id, notes, created_at
		FROM sessions WHERE id = $1
	`, id).Scan(&s.ID, &s.PatientID, &s.ServiceID, &s.PatientPackageID, &s.ScheduledAt, &s.CompletedAt, &s.ConsultantID, &s.Notes, &s.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if err := r.hydrate(ctx, s); err != nil {
		return nil, err
	}
	return s, nil
}

type ListFilter struct {
	PatientID    string
	ConsultantID string
}

// List returns sessions matching the filter, most recent first. Filtering by
// PatientID or ConsultantID is how session history per patient/per
// consultant (per PLAN.md Module 5) is served — no separate endpoints.
func (r *Repository) List(ctx context.Context, filter ListFilter) ([]*Session, error) {
	query := `
		SELECT id, patient_id, service_id, patient_package_id, scheduled_at, completed_at, consultant_id, notes, created_at
		FROM sessions
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
	query += " ORDER BY scheduled_at DESC"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*Session
	for rows.Next() {
		s := &Session{}
		if err := rows.Scan(&s.ID, &s.PatientID, &s.ServiceID, &s.PatientPackageID, &s.ScheduledAt, &s.CompletedAt, &s.ConsultantID, &s.Notes, &s.CreatedAt); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, s := range sessions {
		if err := r.hydrate(ctx, s); err != nil {
			return nil, err
		}
	}
	return sessions, nil
}

// hydrate fills in a session's attendant roster and commission snapshot.
func (r *Repository) hydrate(ctx context.Context, s *Session) error {
	rows, err := r.pool.Query(ctx, `SELECT attendant_id FROM session_attendants WHERE session_id = $1`, s.ID)
	if err != nil {
		return err
	}
	var attendantIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		attendantIDs = append(attendantIDs, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	s.AttendantIDs = attendantIDs

	snap := &CommissionSnapshot{}
	err = r.pool.QueryRow(ctx, `
		SELECT id, session_id, consultant_id, commission_rate, resolution_source, clinic_amount, consultant_amount, created_at
		FROM session_commission_snapshot WHERE session_id = $1
	`, s.ID).Scan(&snap.ID, &snap.SessionID, &snap.ConsultantID, &snap.CommissionRate, &snap.ResolutionSource, &snap.ClinicAmount, &snap.ConsultantAmount, &snap.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	s.CommissionSnapshot = snap
	return nil
}
