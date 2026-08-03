CREATE TABLE session_attendants (
    session_id   UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    attendant_id UUID NOT NULL REFERENCES attendants(id) ON DELETE RESTRICT,
    PRIMARY KEY (session_id, attendant_id)
);
