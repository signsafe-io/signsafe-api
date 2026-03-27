package model

import "time"

type Contract struct {
	ID             string     `db:"id" json:"id"`
	OrganizationID string     `db:"organization_id" json:"organizationId"`
	UploadedBy     string     `db:"uploaded_by" json:"uploadedBy"`
	Title          string     `db:"title" json:"title"`
	Status         string     `db:"status" json:"status"`
	FilePath       string     `db:"file_path" json:"filePath"`
	FileName       string     `db:"file_name" json:"fileName"`
	FileSize       int64      `db:"file_size" json:"fileSize"`
	FileMimeType   string     `db:"file_mime_type" json:"fileMimeType"`
	Parties        string     `db:"parties" json:"parties"`
	Tags           string     `db:"tags" json:"tags"`
	Language       string     `db:"language" json:"language"`
	ContractType   *string    `db:"contract_type" json:"contractType"`
	SignedAt       *time.Time `db:"signed_at" json:"signedAt"`
	ExpiresAt      *time.Time `db:"expires_at" json:"expiresAt"`
	CreatedAt      time.Time  `db:"created_at" json:"createdAt"`
	UpdatedAt      time.Time  `db:"updated_at" json:"updatedAt"`
}

type IngestionJob struct {
	ID          string     `db:"id" json:"id"`
	ContractID  string     `db:"contract_id" json:"contractId"`
	Status      string     `db:"status" json:"status"`
	Progress    int        `db:"progress" json:"progress"`
	CurrentStep *string    `db:"current_step" json:"currentStep"`
	ErrorMsg    *string    `db:"error_message" json:"errorMessage"`
	StartedAt   *time.Time `db:"started_at" json:"startedAt"`
	CompletedAt *time.Time `db:"completed_at" json:"completedAt"`
	CreatedAt   time.Time  `db:"created_at" json:"createdAt"`
	UpdatedAt   time.Time  `db:"updated_at" json:"updatedAt"`
}

type Clause struct {
	ID           string    `db:"id" json:"id"`
	ContractID   string    `db:"contract_id" json:"contractId"`
	ClauseIndex  int       `db:"clause_index" json:"clauseIndex"`
	Label        *string   `db:"label" json:"label"`
	Content      string    `db:"content" json:"content"`
	PageStart    int       `db:"page_start" json:"pageStart"`
	PageEnd      int       `db:"page_end" json:"pageEnd"`
	AnchorX      *float64  `db:"anchor_x" json:"anchorX"`
	AnchorY      *float64  `db:"anchor_y" json:"anchorY"`
	AnchorWidth  *float64  `db:"anchor_width" json:"anchorWidth"`
	AnchorHeight *float64  `db:"anchor_height" json:"anchorHeight"`
	StartOffset  int       `db:"start_offset" json:"startOffset"`
	EndOffset    int       `db:"end_offset" json:"endOffset"`
	CreatedAt    time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt    time.Time `db:"updated_at" json:"updatedAt"`
}
