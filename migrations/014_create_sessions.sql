CREATE TABLE sessions (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id         UUID NOT NULL REFERENCES patients(id) ON DELETE CASCADE,
    service_id         UUID NOT NULL REFERENCES services(id) ON DELETE RESTRICT,
    patient_package_id UUID REFERENCES patient_packages(id) ON DELETE SET NULL,
    scheduled_at       TIMESTAMPTZ NOT NULL,
    completed_at       TIMESTAMPTZ,
    consultant_id      UUID REFERENCES consultants(id) ON DELETE SET NULL,
    notes              TEXT,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
