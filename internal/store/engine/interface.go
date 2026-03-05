package engine

import (
	"context"

	"ftp-worker/internal/reestr"
)

type AuxStore interface {
	Close(context.Context) error

	ErrNotFound() error

	ListDoneByDir(context.Context, string) ([]*reestr.WorkResult, error)
	ReadByDirOpFpID(context.Context, string, string, string) (*reestr.WorkResult, error)
	Create(context.Context, *reestr.WorkResult) (*reestr.WorkResult, error)
	Update(context.Context, *reestr.WorkResult) error
}
