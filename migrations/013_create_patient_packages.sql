CREATE TABLE patient_packages (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id            UUID NOT NULL REFERENCES patients(id) ON DELETE CASCADE,
    package_id            UUID NOT NULL REFERENCES packages(id) ON DELETE CASCADE,
    principal_consultant  UUID REFERENCES consultants(id) ON DELETE SET NULL,
    sessions_remaining    INTEGER NOT NULL CHECK (sessions_remaining >= 0),
    purchased_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);
