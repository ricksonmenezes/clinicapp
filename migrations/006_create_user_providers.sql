-- Phase 2 (OAuth) scaffolding, unused in Phase 1.
CREATE TABLE user_providers (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider     VARCHAR(50) NOT NULL,
    provider_uid VARCHAR(255) NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, provider_uid)
);

CREATE INDEX idx_user_providers_user_id ON user_providers(user_id);
