package auth

import "testing"

func TestValidatePassword(t *testing.T) {
	cases := []struct {
		password string
		wantErr  bool
	}{
		{"short", true},
		{"exactly8", false},
		{"correcthorsebattery", false},
		{"", true},
	}

	for _, c := range cases {
		err := validatePassword(c.password)
		if (err != nil) != c.wantErr {
			t.Errorf("validatePassword(%q) error = %v, wantErr %v", c.password, err, c.wantErr)
		}
	}
}

func TestNormalizeEmail(t *testing.T) {
	cases := map[string]string{
		"  Foo@Example.COM  ": "foo@example.com",
		"already@lower.com":   "already@lower.com",
	}
	for input, want := range cases {
		if got := normalizeEmail(input); got != want {
			t.Errorf("normalizeEmail(%q) = %q, want %q", input, got, want)
		}
	}
}
