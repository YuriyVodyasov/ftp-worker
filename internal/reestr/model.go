package reestr

import (
	"time"

	"github.com/google/uuid"
)

type AddRow struct {
	FpID          string `csv:"fp_id"`
	SudirID       string `csv:"sudir_id,omitempty"`
	AgreeID       string `csv:"agree_id,omitempty"`
	AgreeDateFrom int64  `csv:"agree_date_from,omitempty"`
	FileName      string `csv:"file_name,omitempty"`
	Result
}

type DelRow struct {
	FpID string `csv:"fp_id"`
	Result
}

type Result struct {
	StatusCode  int    `csv:"status_code"`
	ErrorMesage string `csv:"err_message,omitempty"`
}

type WorkResult struct {
	ID        uuid.UUID
	CreatedAt time.Time
	UpdatedAt *time.Time
	Dir       string `csv:"dir"`
	Operation string
	AddRow
}
