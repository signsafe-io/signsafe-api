package model

import (
	"encoding/json"
	"time"
)

type RiskAnalysis struct {
	ID              string          `db:"id" json:"id"`
	ContractID      string          `db:"contract_id" json:"contractId"`
	RequestedBy     string          `db:"requested_by" json:"requestedBy"`
	Status          string          `db:"status" json:"status"`
	ModelVersion    *string         `db:"model_version" json:"modelVersion"`
	ErrorMsg        *string         `db:"error_message" json:"errorMessage"`
	StartedAt       *time.Time      `db:"started_at" json:"startedAt"`
	CompletedAt     *time.Time      `db:"completed_at" json:"completedAt"`
	DocumentSummary *string         `db:"document_summary" json:"documentSummary,omitempty"`
	OverallRisk     *string         `db:"overall_risk" json:"overallRisk,omitempty"`
	KeyIssues       json.RawMessage `db:"key_issues" json:"keyIssues,omitempty"`
	CreatedAt       time.Time       `db:"created_at" json:"createdAt"`
	UpdatedAt       time.Time       `db:"updated_at" json:"updatedAt"`
}

type ClauseResult struct {
	ID                  string     `db:"id" json:"id"`
	AnalysisID          string     `db:"analysis_id" json:"analysisId"`
	ClauseID            string     `db:"clause_id" json:"clauseId"`
	RiskLevel           string     `db:"risk_level" json:"riskLevel"`
	Confidence          float64    `db:"confidence" json:"confidence"`
	IssueType           *string    `db:"issue_type" json:"issueType"`
	Summary             *string    `db:"summary" json:"summary"`
	HighlightX          *float64   `db:"highlight_x" json:"highlightX"`
	HighlightY          *float64   `db:"highlight_y" json:"highlightY"`
	HighlightWidth      *float64   `db:"highlight_width" json:"highlightWidth"`
	HighlightHeight     *float64   `db:"highlight_height" json:"highlightHeight"`
	PageNumber          *int       `db:"page_number" json:"pageNumber"`
	OverriddenRiskLevel *string    `db:"overridden_risk_level" json:"overriddenRiskLevel"`
	OverrideReason      *string    `db:"override_reason" json:"overrideReason"`
	OverriddenBy        *string    `db:"overridden_by" json:"overriddenBy"`
	OverriddenAt        *time.Time `db:"overridden_at" json:"overriddenAt"`
	CreatedAt           time.Time  `db:"created_at" json:"createdAt"`
	UpdatedAt           time.Time  `db:"updated_at" json:"updatedAt"`
}

type EvidenceSet struct {
	ID                 string    `db:"id" json:"id"`
	ClauseResultID     string    `db:"clause_result_id" json:"clauseResultId"`
	Rationale          string    `db:"rationale" json:"rationale"`
	Citations          string    `db:"citations" json:"citations"`
	RecommendedActions string    `db:"recommended_actions" json:"recommendedActions"`
	TopK               int       `db:"top_k" json:"topK"`
	FilterParams       string    `db:"filter_params" json:"filterParams"`
	RetrievedAt        time.Time `db:"retrieved_at" json:"retrievedAt"`
	CreatedAt          time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt          time.Time `db:"updated_at" json:"updatedAt"`
}

type RiskOverride struct {
	ID                string    `db:"id" json:"id"`
	ClauseResultID    string    `db:"clause_result_id" json:"clauseResultId"`
	OriginalRiskLevel string    `db:"original_risk_level" json:"originalRiskLevel"`
	NewRiskLevel      string    `db:"new_risk_level" json:"newRiskLevel"`
	Reason            string    `db:"reason" json:"reason"`
	DecidedBy         string    `db:"decided_by" json:"decidedBy"`
	CreatedAt         time.Time `db:"created_at" json:"createdAt"`
}
