package auth

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

func (r *Repository) CreateUser(ctx context.Context, email, passwordHash, role string) (*User, error) {
	return r.createUser(ctx, email, passwordHash, role, nil, nil)
}

// CreateStaffUser is CreateUser plus the fullName/dob captured on the New
// Staff Account form — used only by RegisterStaff. Kept as a separate
// method rather than adding two more params to every CreateUser call site
// (Register, the test harness, ...), since only staff accounts ever set
// these.
func (r *Repository) CreateStaffUser(ctx context.Context, email, passwordHash, role, fullName string, dob *time.Time) (*User, error) {
	return r.createUser(ctx, email, passwordHash, role, &fullName, dob)
}

func (r *Repository) createUser(ctx context.Context, email, passwordHash, role string, fullName *string, dob *time.Time) (*User, error) {
	u := &User{}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, role, status, full_name, date_of_birth)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, email, password_hash, role, status, full_name, date_of_birth, created_at, updated_at
	`, email, passwordHash, role, StatusUnverified, fullName, dob).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.Status, &u.FullName, &u.DateOfBirth, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			return nil, ErrEmailTaken
		}
		return nil, err
	}
	return u, nil
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	u := &User{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, email, password_hash, role, status, full_name, date_of_birth, created_at, updated_at
		FROM users WHERE email = $1
	`, email).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.Status, &u.FullName, &u.DateOfBirth, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return u, nil
}

func (r *Repository) GetUserByID(ctx context.Context, id string) (*User, error) {
	u := &User{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, email, password_hash, role, status, full_name, date_of_birth, created_at, updated_at
		FROM users WHERE id = $1
	`, id).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.Status, &u.FullName, &u.DateOfBirth, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return u, nil
}

// SearchUsers looks up staff accounts (clinician/attendant/admin) by a
// case-insensitive full_name substring, for the backoffice's "pick a staff
// account by name" pickers (GET /users) — replaces having to paste a raw
// user id copied from the one-time RegisterStaff confirmation message.
// unlinkedOnly, when true, additionally excludes users who already have a
// consultant/attendant profile row (role-dependent; a no-op for role=admin,
// which has no profile table). Reaches into the consultants/attendants
// tables directly via a LEFT JOIN rather than adding a cross-package
// querier abstraction, same precedent as session.Repository reaching into
// patient_packages (see M4's decision log).
func (r *Repository) SearchUsers(ctx context.Context, role, query string, unlinkedOnly bool) ([]*User, error) {
	joinTable := ""
	switch role {
	case RoleClinician:
		joinTable = "consultants"
	case RoleAttendant:
		joinTable = "attendants"
	}

	sql := `SELECT u.id, u.email, u.full_name, u.date_of_birth FROM users u`
	if joinTable != "" {
		sql += ` LEFT JOIN ` + joinTable + ` p ON p.user_id = u.id`
	}
	sql += ` WHERE u.role = $1 AND u.full_name ILIKE '%' || $2 || '%'`
	if unlinkedOnly && joinTable != "" {
		sql += ` AND p.id IS NULL`
	}
	sql += ` ORDER BY u.full_name LIMIT 20`

	rows, err := r.pool.Query(ctx, sql, role, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		u := &User{Role: role}
		if err := rows.Scan(&u.ID, &u.Email, &u.FullName, &u.DateOfBirth); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (r *Repository) ActivateUser(ctx context.Context, userID string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE users SET status = $1, updated_at = now() WHERE id = $2
	`, StatusActive, userID)
	return err
}

func (r *Repository) UpdateUserPassword(ctx context.Context, userID, passwordHash string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE users SET password_hash = $1, updated_at = now() WHERE id = $2
	`, passwordHash, userID)
	return err
}

func (r *Repository) CreateEmailVerificationToken(ctx context.Context, userID string, ttl time.Duration) (*EmailVerificationToken, error) {
	t := &EmailVerificationToken{}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO email_verification_tokens (user_id, expires_at)
		VALUES ($1, $2)
		RETURNING id, user_id, token, expires_at, used_at
	`, userID, time.Now().Add(ttl)).Scan(&t.ID, &t.UserID, &t.Token, &t.ExpiresAt, &t.UsedAt)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (r *Repository) GetEmailVerificationToken(ctx context.Context, token string) (*EmailVerificationToken, error) {
	t := &EmailVerificationToken{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, token, expires_at, used_at
		FROM email_verification_tokens WHERE token = $1
	`, token).Scan(&t.ID, &t.UserID, &t.Token, &t.ExpiresAt, &t.UsedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTokenInvalid
		}
		return nil, err
	}
	return t, nil
}

func (r *Repository) MarkEmailVerificationTokenUsed(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE email_verification_tokens SET used_at = now() WHERE id = $1
	`, id)
	return err
}

func (r *Repository) CreateRefreshToken(ctx context.Context, userID string, ttl time.Duration) (*RefreshToken, error) {
	t := &RefreshToken{}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO refresh_tokens (user_id, expires_at)
		VALUES ($1, $2)
		RETURNING id, user_id, token, expires_at, revoked_at
	`, userID, time.Now().Add(ttl)).Scan(&t.ID, &t.UserID, &t.Token, &t.ExpiresAt, &t.RevokedAt)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (r *Repository) GetRefreshToken(ctx context.Context, token string) (*RefreshToken, error) {
	t := &RefreshToken{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, token, expires_at, revoked_at
		FROM refresh_tokens WHERE token = $1
	`, token).Scan(&t.ID, &t.UserID, &t.Token, &t.ExpiresAt, &t.RevokedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTokenInvalid
		}
		return nil, err
	}
	return t, nil
}

func (r *Repository) RevokeRefreshToken(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = now() WHERE id = $1
	`, id)
	return err
}

func (r *Repository) RevokeAllRefreshTokensForUser(ctx context.Context, userID string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL
	`, userID)
	return err
}

func (r *Repository) CreatePasswordResetToken(ctx context.Context, userID string, ttl time.Duration) (*PasswordResetToken, error) {
	t := &PasswordResetToken{}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO password_reset_tokens (user_id, expires_at)
		VALUES ($1, $2)
		RETURNING id, user_id, token, expires_at, used_at
	`, userID, time.Now().Add(ttl)).Scan(&t.ID, &t.UserID, &t.Token, &t.ExpiresAt, &t.UsedAt)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (r *Repository) GetPasswordResetToken(ctx context.Context, token string) (*PasswordResetToken, error) {
	t := &PasswordResetToken{}
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, token, expires_at, used_at
		FROM password_reset_tokens WHERE token = $1
	`, token).Scan(&t.ID, &t.UserID, &t.Token, &t.ExpiresAt, &t.UsedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTokenInvalid
		}
		return nil, err
	}
	return t, nil
}

func (r *Repository) MarkPasswordResetTokenUsed(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE password_reset_tokens SET used_at = now() WHERE id = $1
	`, id)
	return err
}
