package service

import (
	"context"

	"ftp-worker/internal/reestr"
	"ftp-worker/internal/store/engine"
)

type Store struct {
	engine engine.AuxStore
}

func New(engine engine.AuxStore) *Store {
	return &Store{engine: engine}
}

func (s Store) Close(ctx context.Context) error {
	return s.engine.Close(ctx)
}

func (s Store) ErrNotFound() error {
	return s.engine.ErrNotFound()
}

func (s Store) ListDoneByDir(ctx context.Context, dir string) ([]*reestr.WorkResult, error) {
	return s.engine.ListDoneByDir(ctx, dir)
}

func (s Store) ReadByDirOpFpID(ctx context.Context, fpID, dir, op string) (*reestr.WorkResult, error) {
	return s.engine.ReadByDirOpFpID(ctx, fpID, dir, op)
}

func (s Store) Create(ctx context.Context, workResult *reestr.WorkResult) (*reestr.WorkResult, error) {
	return s.engine.Create(ctx, workResult)
}

func (s Store) Update(ctx context.Context, workResult *reestr.WorkResult) error {
	return s.engine.Update(ctx, workResult)
}
