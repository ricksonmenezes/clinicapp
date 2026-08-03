CREATE TABLE consultants (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id            UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    full_name          VARCHAR(255) NOT NULL,
    default_commission NUMERIC(5,2) NOT NULL DEFAULT 0 CHECK (default_commission >= 0 AND default_commission <= 100),
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
