package store

import "testing"

func TestSplitStatements_StripsCommentsIncludingEmbeddedSemicolons(t *testing.T) {
	sql := `-- Phase 2 (OAuth) scaffolding; unused in Phase 1.
CREATE TABLE user_providers (
    id UUID PRIMARY KEY
);

CREATE INDEX idx_user_providers_user_id ON user_providers(user_id);
`

	stmts := splitStatements(sql)
	if len(stmts) != 2 {
		t.Fatalf("got %d statements, want 2: %#v", len(stmts), stmts)
	}
	for _, s := range stmts {
		if s == "" {
			t.Errorf("statement is empty")
		}
	}
	if want := "CREATE TABLE user_providers"; len(stmts[0]) < len(want) || stmts[0][:len(want)] != want {
		t.Errorf("first statement = %q, want prefix %q", stmts[0], want)
	}
	if want := "CREATE INDEX idx_user_providers_user_id"; len(stmts[1]) < len(want) || stmts[1][:len(want)] != want {
		t.Errorf("second statement = %q, want prefix %q", stmts[1], want)
	}
}

func TestSplitStatements_IgnoresEmptyAndCommentOnlySegments(t *testing.T) {
	sql := `
-- just a comment
CREATE EXTENSION IF NOT EXISTS pgcrypto;
`
	stmts := splitStatements(sql)
	if len(stmts) != 1 {
		t.Fatalf("got %d statements, want 1: %#v", len(stmts), stmts)
	}
}
