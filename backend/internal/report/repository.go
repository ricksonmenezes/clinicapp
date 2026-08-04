package report

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// RevenueByPeriod sums invoice totals by issued_at bucket. groupBy must
// already be validated by the caller (Service) against the day/week/month
// allow-list — it's passed straight into date_trunc as a bind parameter.
func (r *Repository) RevenueByPeriod(ctx context.Context, start, end time.Time, groupBy string) ([]RevenuePoint, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT date_trunc($3, issued_at) AS period, COALESCE(SUM(total), 0)
		FROM invoices
		WHERE issued_at >= $1 AND issued_at < $2
		GROUP BY period
		ORDER BY period
	`, start, end, groupBy)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []RevenuePoint
	for rows.Next() {
		var p RevenuePoint
		if err := rows.Scan(&p.Period, &p.Total); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	return points, rows.Err()
}

// CommissionPayouts sums each consultant's earned commission (and the
// clinic's matching cut) over every session_commission_snapshot recorded in
// [start, end), most-earned first.
func (r *Repository) CommissionPayouts(ctx context.Context, start, end time.Time) ([]CommissionPayout, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT c.id, c.full_name, COALESCE(SUM(scs.consultant_amount), 0), COALESCE(SUM(scs.clinic_amount), 0), COUNT(*)
		FROM session_commission_snapshot scs
		JOIN consultants c ON c.id = scs.consultant_id
		WHERE scs.created_at >= $1 AND scs.created_at < $2
		GROUP BY c.id, c.full_name
		ORDER BY SUM(scs.consultant_amount) DESC
	`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var payouts []CommissionPayout
	for rows.Next() {
		var p CommissionPayout
		if err := rows.Scan(&p.ConsultantID, &p.ConsultantName, &p.ConsultantAmount, &p.ClinicAmount, &p.SessionCount); err != nil {
			return nil, err
		}
		payouts = append(payouts, p)
	}
	return payouts, rows.Err()
}

// ServicePopularity counts sessions per service scheduled in [start, end),
// most-booked first.
func (r *Repository) ServicePopularity(ctx context.Context, start, end time.Time) ([]ServicePopularity, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT sv.id, sv.name, COUNT(*)
		FROM sessions s
		JOIN services sv ON sv.id = s.service_id
		WHERE s.scheduled_at >= $1 AND s.scheduled_at < $2
		GROUP BY sv.id, sv.name
		ORDER BY COUNT(*) DESC
	`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var popularity []ServicePopularity
	for rows.Next() {
		var p ServicePopularity
		if err := rows.Scan(&p.ServiceID, &p.ServiceName, &p.SessionCount); err != nil {
			return nil, err
		}
		popularity = append(popularity, p)
	}
	return popularity, rows.Err()
}

// BookingVolume counts every session (not just self-service bookings) by
// scheduled_at bucket. groupBy is pre-validated the same way as
// RevenueByPeriod's.
func (r *Repository) BookingVolume(ctx context.Context, start, end time.Time, groupBy string) ([]BookingVolumePoint, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT date_trunc($3, scheduled_at) AS period, COUNT(*)
		FROM sessions
		WHERE scheduled_at >= $1 AND scheduled_at < $2
		GROUP BY period
		ORDER BY period
	`, start, end, groupBy)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []BookingVolumePoint
	for rows.Next() {
		var p BookingVolumePoint
		if err := rows.Scan(&p.Period, &p.Count); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	return points, rows.Err()
}
