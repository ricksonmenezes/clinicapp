CREATE TABLE packages (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id    UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    name          VARCHAR(255) NOT NULL,
    session_count INTEGER NOT NULL CHECK (session_count > 0),
    price         NUMERIC(10,2) NOT NULL CHECK (price >= 0),
    active        BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
