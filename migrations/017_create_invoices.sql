CREATE TABLE invoices (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID REFERENCES sessions(id) ON DELETE SET NULL,
    package_id UUID REFERENCES patient_packages(id) ON DELETE SET NULL,
    patient_id UUID NOT NULL REFERENCES patients(id) ON DELETE CASCADE,
    total      NUMERIC(10,2) NOT NULL CHECK (total >= 0),
    issued_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    pdf_path   VARCHAR(500),
    CHECK (
        (session_id IS NOT NULL AND package_id IS NULL) OR
        (session_id IS NULL AND package_id IS NOT NULL)
    )
);
