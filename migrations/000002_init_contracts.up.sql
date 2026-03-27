-- 000002_init_contracts.up.sql
-- Contracts, Ingestion Jobs, Clauses

CREATE TABLE contracts (
    id              VARCHAR(26)  PRIMARY KEY,
    organization_id VARCHAR(26)  NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    uploaded_by     VARCHAR(26)  NOT NULL REFERENCES users(id),
    title           VARCHAR(500) NOT NULL,
    status          VARCHAR(50)  NOT NULL DEFAULT 'uploaded',
    -- status: uploaded | processing | processed | failed
    file_path       VARCHAR(1000) NOT NULL,
    file_name       VARCHAR(500) NOT NULL,
    file_size       BIGINT       NOT NULL DEFAULT 0,
    file_mime_type  VARCHAR(100) NOT NULL DEFAULT 'application/pdf',
    parties         JSONB        NOT NULL DEFAULT '[]',
    tags            JSONB        NOT NULL DEFAULT '[]',
    language        VARCHAR(10)  NOT NULL DEFAULT 'ko',
    contract_type   VARCHAR(100),
    signed_at       TIMESTAMPTZ,
    expires_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_contracts_organization_id ON contracts(organization_id);
CREATE INDEX idx_contracts_uploaded_by ON contracts(uploaded_by);
CREATE INDEX idx_contracts_status ON contracts(status);
CREATE INDEX idx_contracts_created_at ON contracts(created_at DESC);

CREATE TABLE ingestion_jobs (
    id              VARCHAR(26) PRIMARY KEY,
    contract_id     VARCHAR(26) NOT NULL REFERENCES contracts(id) ON DELETE CASCADE,
    status          VARCHAR(50) NOT NULL DEFAULT 'pending',
    -- status: pending | parsing | chunking | indexing | completed | failed
    progress        INTEGER     NOT NULL DEFAULT 0 CHECK (progress >= 0 AND progress <= 100),
    current_step    VARCHAR(100),
    error_message   TEXT,
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ingestion_jobs_contract_id ON ingestion_jobs(contract_id);
CREATE INDEX idx_ingestion_jobs_status ON ingestion_jobs(status);

CREATE TABLE clauses (
    id              VARCHAR(26)  PRIMARY KEY,
    contract_id     VARCHAR(26)  NOT NULL REFERENCES contracts(id) ON DELETE CASCADE,
    clause_index    INTEGER      NOT NULL,
    label           VARCHAR(255),
    content         TEXT         NOT NULL,
    page_start      INTEGER      NOT NULL DEFAULT 1,
    page_end        INTEGER      NOT NULL DEFAULT 1,
    -- anchor: bounding box coordinates on the page
    anchor_x        FLOAT,
    anchor_y        FLOAT,
    anchor_width    FLOAT,
    anchor_height   FLOAT,
    start_offset    INTEGER      NOT NULL DEFAULT 0,
    end_offset      INTEGER      NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_clauses_contract_id ON clauses(contract_id);
CREATE INDEX idx_clauses_contract_index ON clauses(contract_id, clause_index);
