package auth

import (
	"testing"
	"time"
)

func TestNewAccessToken_RoundTrip(t *testing.T) {
	tokenString, err := NewAccessToken("test-secret", 15*time.Minute, "user-123", RoleClinician)
	if err != nil {
		t.Fatalf("NewAccessToken: %v", err)
	}

	claims, err := ParseAccessToken("test-secret", tokenString)
	if err != nil {
		t.Fatalf("ParseAccessToken: %v", err)
	}
	if claims.UserID != "user-123" {
		t.Errorf("UserID = %q, want %q", claims.UserID, "user-123")
	}
	if claims.Role != RoleClinician {
		t.Errorf("Role = %q, want %q", claims.Role, RoleClinician)
	}
}

func TestParseAccessToken_ExpiredRejected(t *testing.T) {
	tokenString, err := NewAccessToken("test-secret", -1*time.Minute, "user-123", RolePatient)
	if err != nil {
		t.Fatalf("NewAccessToken: %v", err)
	}

	if _, err := ParseAccessToken("test-secret", tokenString); err == nil {
		t.Fatal("expected error parsing expired token, got nil")
	}
}

func TestParseAccessToken_WrongSecretRejected(t *testing.T) {
	tokenString, err := NewAccessToken("test-secret", 15*time.Minute, "user-123", RolePatient)
	if err != nil {
		t.Fatalf("NewAccessToken: %v", err)
	}

	if _, err := ParseAccessToken("wrong-secret", tokenString); err == nil {
		t.Fatal("expected error parsing token signed with a different secret, got nil")
	}
}
