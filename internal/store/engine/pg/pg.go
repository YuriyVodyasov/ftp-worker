package pg

import (
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pkg/errors"

	"ftp-worker/internal/reestr"
)

type PG struct {
	errorProcessing bool
	DB              *pgxpool.Pool
}

func New(ctx context.Context, cfg *PGPoolCfg, errorProcessing bool) (*PG, error) {
	dbParams, err := cfg.BuildConnectionURL()
	if err != nil {
		return nil, errors.Wrap(err, "failed to build connection URL")
	}

	poolConfig, err := pgxpool.ParseConfig(dbParams)
	if err != nil {
		return nil, errors.Wrap(err, "unable to parse config")
	}

	connection, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to postgres database")
	}

	if err = connection.Ping(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to connect to postgres database")
	}

	return &PG{
		DB:              connection,
		errorProcessing: errorProcessing,
	}, nil
}

func (p *PG) Close(_ context.Context) error {
	p.DB.Close()

	return nil
}

func (p *PG) ErrNotFound() error {
	return sql.ErrNoRows
}

func (p *PG) ListDoneByDir(ctx context.Context, rootEntryName string) ([]*reestr.WorkResult, error) {
	query := "SELECT * FROM records WHERE dir = $1"

	if p.errorProcessing {
		query += " and (status_code = 202 OR status_code = 200 OR status_code = 410 OR status_code = 400);"
	}

	rows, err := p.DB.Query(ctx, query, rootEntryName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to query done list")
	}

	defer rows.Close()

	var results []*reestr.WorkResult

	for rows.Next() {
		var wr reestr.WorkResult

		err := rows.Scan(&wr.ID,
			&wr.CreatedAt,
			&wr.UpdatedAt,
			&wr.Dir,
			&wr.Operation,
			&wr.FpID,
			&wr.SudirID,
			&wr.AgreeID,
			&wr.AgreeDateFrom,
			&wr.FileName,
			&wr.StatusCode,
			&wr.ErrorMesage)

		if err != nil {
			return nil, errors.Wrap(err, "failed to scan row")
		}

		results = append(results, &wr)
	}

	return results, nil
}

func (p *PG) ReadByDirOpFpID(ctx context.Context, id, dir, op string) (*reestr.WorkResult, error) {
	query := "SELECT * FROM records WHERE fp_id = $1 and dir = $2 and operation = $3;"

	row := p.DB.QueryRow(ctx, query, id, dir, op)

	var wr reestr.WorkResult

	err := row.Scan(&wr.ID,
		&wr.CreatedAt,
		&wr.UpdatedAt,
		&wr.Dir,
		&wr.Operation,
		&wr.FpID,
		&wr.SudirID,
		&wr.AgreeID,
		&wr.AgreeDateFrom,
		&wr.FileName,
		&wr.StatusCode,
		&wr.ErrorMesage)

	if err != nil {
		return nil, errors.Wrap(err, "failed to scan row")
	}

	return &wr, nil
}

func (p *PG) Create(ctx context.Context, wr *reestr.WorkResult) (*reestr.WorkResult, error) {
	query := `INSERT INTO records(
		dir,
		operation,
		fp_id,
		sudir_id,
		agree_id,
		agree_date_from,
		file_name,
		status_code,
		err_message,
		created_at
		)
		VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW() at time zone 'utc') returning *;`

	row := p.DB.QueryRow(ctx, query,
		wr.Dir,
		wr.Operation,
		wr.FpID,
		wr.SudirID,
		wr.AgreeID,
		wr.AgreeDateFrom,
		wr.FileName,
		wr.StatusCode,
		wr.ErrorMesage)

	var created reestr.WorkResult

	err := row.Scan(&created.ID,
		&created.CreatedAt,
		&created.UpdatedAt,
		&created.Dir,
		&created.Operation,
		&created.FpID,
		&created.SudirID,
		&created.AgreeID,
		&created.AgreeDateFrom,
		&created.FileName,
		&created.StatusCode,
		&created.ErrorMesage)

	if err != nil {
		return nil, errors.Wrap(err, "failed to scan row")
	}

	return &created, nil
}

func (p *PG) Update(ctx context.Context, wr *reestr.WorkResult) error {
	query := `UPDATE records SET
		sudir_id = $2,
		agree_id = $3,
		agree_date_from = $4,
		status_code = $5,
		err_message = $6,
		updated_at = NOW() at time zone 'utc' WHERE id = $1;`

	_, err := p.DB.Exec(ctx, query,
		wr.ID,
		wr.SudirID,
		wr.AgreeID,
		wr.AgreeDateFrom,
		wr.StatusCode,
		wr.ErrorMesage)

	return errors.Wrap(err, "failed to update row")
}
