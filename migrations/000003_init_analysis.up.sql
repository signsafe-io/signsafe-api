-- 000003_init_analysis.up.sql
-- Risk Analyses, Clause Results, Evidence Sets, Risk Overrides

CREATE TABLE risk_analyses (
    id              VARCHAR(26) PRIMARY KEY,
    contract_id     VARCHAR(26) NOT NULL REFERENCES contracts(id) ON DELETE CASCADE,
    requested_by    VARCHAR(26) NOT NULL REFERENCES users(id),
    status          VARCHAR(50) NOT NULL DEFAULT 'pending',
    -- status: pending | running | completed | failed
    model_version   VARCHAR(100),
    error_message   TEXT,
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_risk_analyses_contract_id ON risk_analyses(contract_id);
CREATE INDEX idx_risk_analyses_requested_by ON risk_analyses(requested_by);
CREATE INDEX idx_risk_analyses_status ON risk_analyses(status);

CREATE TABLE clause_results (
    id              VARCHAR(26)  PRIMARY KEY,
    analysis_id     VARCHAR(26)  NOT NULL REFERENCES risk_analyses(id) ON DELETE CASCADE,
    clause_id       VARCHAR(26)  NOT NULL REFERENCES clauses(id) ON DELETE CASCADE,
    risk_level      VARCHAR(50)  NOT NULL DEFAULT 'unknown',
    -- risk_level: high | medium | low | safe | unknown
    issue_type      VARCHAR(100),
    summary         TEXT,
    highlight_x     FLOAT,
    highlight_y     FLOAT,
    highlight_width FLOAT,
    highlight_height FLOAT,
    page_number     INTEGER,
    -- overridden fields
    overridden_risk_level VARCHAR(50),
    override_reason       TEXT,
    overridden_by         VARCHAR(26) REFERENCES users(id),
    overridden_at         TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_clause_results_analysis_id ON clause_results(analysis_id);
CREATE INDEX idx_clause_results_clause_id ON clause_results(clause_id);
CREATE INDEX idx_clause_results_risk_level ON clause_results(risk_level);

CREATE TABLE evidence_sets (
    id              VARCHAR(26)  PRIMARY KEY,
    clause_result_id VARCHAR(26) NOT NULL REFERENCES clause_results(id) ON DELETE CASCADE,
    rationale       TEXT         NOT NULL,
    citations       JSONB        NOT NULL DEFAULT '[]',
    recommended_actions JSONB    NOT NULL DEFAULT '[]',
    top_k           INTEGER      NOT NULL DEFAULT 5,
    filter_params   JSONB        NOT NULL DEFAULT '{}',
    retrieved_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_evidence_sets_clause_result_id ON evidence_sets(clause_result_id);

CREATE TABLE risk_overrides (
    id                  VARCHAR(26)  PRIMARY KEY,
    clause_result_id    VARCHAR(26)  NOT NULL REFERENCES clause_results(id) ON DELETE CASCADE,
    original_risk_level VARCHAR(50)  NOT NULL,
    new_risk_level      VARCHAR(50)  NOT NULL,
    reason              TEXT         NOT NULL,
    decided_by          VARCHAR(26)  NOT NULL REFERENCES users(id),
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_risk_overrides_clause_result_id ON risk_overrides(clause_result_id);
CREATE INDEX idx_risk_overrides_decided_by ON risk_overrides(decided_by);
