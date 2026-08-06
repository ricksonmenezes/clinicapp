package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"clinicapp/backend/internal/mailer"
)

const bcryptCost = 12

const (
	emailVerificationTTL = 24 * time.Hour
	passwordResetTTL     = 1 * time.Hour
)

type ServiceConfig struct {
	JWTSecret          string
	JWTExpiry          time.Duration
	RefreshTokenExpiry time.Duration
	BaseURL            string
}

type Service struct {
	repo   *Repository
	mailer mailer.Mailer
	cfg    ServiceConfig
}

func NewService(repo *Repository, m mailer.Mailer, cfg ServiceConfig) *Service {
	return &Service{repo: repo, mailer: m, cfg: cfg}
}

// TokenPair is what every flow that issues credentials returns.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int // access token TTL, seconds
}

// Register is the public self-service signup path — it always creates a
// patient account. Elevated roles can only be created via RegisterStaff by
// an already-authenticated admin, so a request body can never self-assign
// admin/clinician/attendant.
func (s *Service) Register(ctx context.Context, email, password string) (*User, error) {
	email = normalizeEmail(email)
	if err := validatePassword(password); err != nil {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return nil, err
	}

	user, err := s.repo.CreateUser(ctx, email, string(hash), RolePatient)
	if err != nil {
		return nil, err
	}

	if err := s.sendVerificationEmail(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

// RegisterStaff creates a clinician/attendant/admin account. Callers must
// enforce (via middleware) that only an authenticated admin can reach this —
// the service layer only enforces that the role itself is a valid staff
// role. The account is activated immediately since an admin is vouching for
// it; no email verification round-trip.
//
// fullName/dob exist purely so the account can later be found by name via
// SearchUsers instead of by pasting the raw id shown once in this call's
// response — they're not used anywhere in the auth flow itself. dob is
// optional (only useful as a disambiguator when two staff share a name).
func (s *Service) RegisterStaff(ctx context.Context, email, password, role, fullName string, dob *time.Time) (*User, error) {
	email = normalizeEmail(email)
	if err := validatePassword(password); err != nil {
		return nil, err
	}
	switch role {
	case RoleClinician, RoleAttendant, RoleAdmin:
	default:
		return nil, ErrInvalidRole
	}
	fullName = strings.TrimSpace(fullName)
	if fullName == "" {
		return nil, ErrFullNameRequired
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return nil, err
	}

	user, err := s.repo.CreateStaffUser(ctx, email, string(hash), role, fullName, dob)
	if err != nil {
		return nil, err
	}

	if err := s.repo.ActivateUser(ctx, user.ID); err != nil {
		return nil, err
	}
	user.Status = StatusActive

	return user, nil
}

// SearchUsers looks up staff accounts by name for the backoffice's
// name-search pickers (GET /users). See Repository.SearchUsers for the
// unlinkedOnly semantics.
func (s *Service) SearchUsers(ctx context.Context, role, query string, unlinkedOnly bool) ([]*User, error) {
	switch role {
	case RoleClinician, RoleAttendant, RoleAdmin:
	default:
		return nil, ErrInvalidRole
	}
	return s.repo.SearchUsers(ctx, role, query, unlinkedOnly)
}

// EnsureBootstrapAdmin creates the given admin account if no user with that
// email exists yet, and is a no-op otherwise — safe to call on every server
// startup. It exists solely to break the chicken-and-egg problem of
// RegisterStaff requiring an already-authenticated admin.
func (s *Service) EnsureBootstrapAdmin(ctx context.Context, email, password string) error {
	email = normalizeEmail(email)
	if _, err := s.repo.GetUserByEmail(ctx, email); err == nil {
		return nil
	} else if !errors.Is(err, ErrUserNotFound) {
		return err
	}

	if err := validatePassword(password); err != nil {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return err
	}

	user, err := s.repo.CreateUser(ctx, email, string(hash), RoleAdmin)
	if err != nil {
		return err
	}

	return s.repo.ActivateUser(ctx, user.ID)
}

func (s *Service) sendVerificationEmail(ctx context.Context, user *User) error {
	tok, err := s.repo.CreateEmailVerificationToken(ctx, user.ID, emailVerificationTTL)
	if err != nil {
		return err
	}

	link := fmt.Sprintf("%s/auth/verify-email?token=%s", s.cfg.BaseURL, tok.Token)
	return s.mailer.Send(ctx, mailer.Message{
		To:      user.Email,
		Subject: "Verify your clinicapp account",
		Body:    fmt.Sprintf("Verify your email by visiting: %s\n\nThis link expires in 24 hours.", link),
	})
}

// ResendVerification silently no-ops for unknown or already-verified emails
// so the endpoint can't be used to enumerate registered addresses.
func (s *Service) ResendVerification(ctx context.Context, email string) error {
	user, err := s.repo.GetUserByEmail(ctx, normalizeEmail(email))
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil
		}
		return err
	}
	if user.Status != StatusUnverified {
		return nil
	}
	return s.sendVerificationEmail(ctx, user)
}

func (s *Service) VerifyEmail(ctx context.Context, token string) (*User, *TokenPair, error) {
	vt, err := s.repo.GetEmailVerificationToken(ctx, token)
	if err != nil {
		return nil, nil, err
	}
	if vt.UsedAt != nil {
		return nil, nil, ErrTokenUsed
	}
	if time.Now().After(vt.ExpiresAt) {
		return nil, nil, ErrTokenExpired
	}

	if err := s.repo.ActivateUser(ctx, vt.UserID); err != nil {
		return nil, nil, err
	}
	if err := s.repo.MarkEmailVerificationTokenUsed(ctx, vt.ID); err != nil {
		return nil, nil, err
	}

	user, err := s.repo.GetUserByID(ctx, vt.UserID)
	if err != nil {
		return nil, nil, err
	}

	pair, err := s.issueTokenPair(ctx, user)
	if err != nil {
		return nil, nil, err
	}
	return user, pair, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (*User, *TokenPair, error) {
	user, err := s.repo.GetUserByEmail(ctx, normalizeEmail(email))
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, nil, ErrInvalidCredentials
		}
		return nil, nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, nil, ErrInvalidCredentials
	}

	switch user.Status {
	case StatusUnverified:
		return nil, nil, ErrEmailNotVerified
	case StatusSuspended:
		return nil, nil, ErrUserSuspended
	}

	pair, err := s.issueTokenPair(ctx, user)
	if err != nil {
		return nil, nil, err
	}
	return user, pair, nil
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	rt, err := s.repo.GetRefreshToken(ctx, refreshToken)
	if err != nil {
		return nil, err
	}
	if rt.RevokedAt != nil {
		return nil, ErrTokenRevoked
	}
	if time.Now().After(rt.ExpiresAt) {
		return nil, ErrTokenExpired
	}

	user, err := s.repo.GetUserByID(ctx, rt.UserID)
	if err != nil {
		return nil, err
	}

	if err := s.repo.RevokeRefreshToken(ctx, rt.ID); err != nil {
		return nil, err
	}

	return s.issueTokenPair(ctx, user)
}

func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	rt, err := s.repo.GetRefreshToken(ctx, refreshToken)
	if err != nil {
		if errors.Is(err, ErrTokenInvalid) {
			return nil
		}
		return err
	}
	if rt.RevokedAt != nil {
		return nil
	}
	return s.repo.RevokeRefreshToken(ctx, rt.ID)
}

// ForgotPassword always returns nil for unknown emails, matching the
// no-enumeration contract in PLAN.md.
func (s *Service) ForgotPassword(ctx context.Context, email string) error {
	user, err := s.repo.GetUserByEmail(ctx, normalizeEmail(email))
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil
		}
		return err
	}

	tok, err := s.repo.CreatePasswordResetToken(ctx, user.ID, passwordResetTTL)
	if err != nil {
		return err
	}

	link := fmt.Sprintf("%s/reset-password?token=%s", s.cfg.BaseURL, tok.Token)
	return s.mailer.Send(ctx, mailer.Message{
		To:      user.Email,
		Subject: "Reset your clinicapp password",
		Body:    fmt.Sprintf("Reset your password by visiting: %s\n\nThis link expires in 1 hour.", link),
	})
}

func (s *Service) ResetPassword(ctx context.Context, token, newPassword string) error {
	if err := validatePassword(newPassword); err != nil {
		return err
	}

	pt, err := s.repo.GetPasswordResetToken(ctx, token)
	if err != nil {
		return err
	}
	if pt.UsedAt != nil {
		return ErrTokenUsed
	}
	if time.Now().After(pt.ExpiresAt) {
		return ErrTokenExpired
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcryptCost)
	if err != nil {
		return err
	}

	if err := s.repo.UpdateUserPassword(ctx, pt.UserID, string(hash)); err != nil {
		return err
	}
	if err := s.repo.MarkPasswordResetTokenUsed(ctx, pt.ID); err != nil {
		return err
	}
	if err := s.repo.RevokeAllRefreshTokensForUser(ctx, pt.UserID); err != nil {
		return err
	}

	user, err := s.repo.GetUserByID(ctx, pt.UserID)
	if err != nil {
		return err
	}
	return s.mailer.Send(ctx, mailer.Message{
		To:      user.Email,
		Subject: "Your clinicapp password was changed",
		Body:    "Your password was just changed. If this wasn't you, contact support immediately.",
	})
}

func (s *Service) issueTokenPair(ctx context.Context, user *User) (*TokenPair, error) {
	access, err := NewAccessToken(s.cfg.JWTSecret, s.cfg.JWTExpiry, user.ID, user.Role)
	if err != nil {
		return nil, err
	}

	refresh, err := s.repo.CreateRefreshToken(ctx, user.ID, s.cfg.RefreshTokenExpiry)
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  access,
		RefreshToken: refresh.Token,
		ExpiresIn:    int(s.cfg.JWTExpiry.Seconds()),
	}, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func validatePassword(password string) error {
	if len(password) < 8 {
		return ErrWeakPassword
	}
	return nil
}
