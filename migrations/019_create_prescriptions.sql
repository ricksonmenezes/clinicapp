CREATE TABLE prescriptions (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id    UUID REFERENCES sessions(id) ON DELETE SET NULL,
    consultant_id UUID NOT NULL REFERENCES consultants(id) ON DELETE CASCADE,
    patient_id    UUID NOT NULL REFERENCES patients(id) ON DELETE CASCADE,
    content       TEXT NOT NULL,
    issued_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    pdf_path      VARCHAR(500)
);
