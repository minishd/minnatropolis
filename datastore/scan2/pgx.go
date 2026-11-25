package scan2

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func ScanOne2[T any](ctx context.Context, q Querier, sql string, args ...any) (*T, error) {
	row := q.QueryRow(ctx, sql, args...)

	v, err := Scan2[T](row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return v, nil
}

func ScanMany2[T any](ctx context.Context, q Querier, sql string, args ...any) ([]*T, error) {
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var values []*T
	for rows.Next() {
		value, err := Scan2[T](rows)
		if err != nil {
			return nil, err
		}

		values = append(values, value)
	}

	return values, nil
}
