CREATE TABLE session_commission_snapshot (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id        UUID NOT NULL UNIQUE REFERENCES sessions(id) ON DELETE CASCADE,
    consultant_id     UUID NOT NULL REFERENCES consultants(id) ON DELETE RESTRICT,
    commission_rate   NUMERIC(5,2) NOT NULL CHECK (commission_rate >= 0 AND commission_rate <= 100),
    resolution_source VARCHAR(30) NOT NULL CHECK (resolution_source IN ('session_override', 'service_override', 'consultant_default')),
    clinic_amount     NUMERIC(10,2) NOT NULL,
    consultant_amount NUMERIC(10,2) NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
