CREATE TABLE consultant_service_commission (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    consultant_id UUID NOT NULL REFERENCES consultants(id) ON DELETE CASCADE,
    service_id    UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    commission    NUMERIC(5,2) NOT NULL CHECK (commission >= 0 AND commission <= 100),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (consultant_id, service_id)
);
