package harness

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func RowAsStrings(ctx context.Context, t require.TestingT, pool *pgxpool.Pool, table, id string) map[string]string {
	rows, err := pool.Query(ctx, fmt.Sprintf(`SELECT * FROM %s WHERE id = $1`, table), id)
	require.NoError(t, err)
	defer rows.Close()
	require.True(t, rows.Next(), "no row for id=%s in %s", id, table)

	fds := rows.FieldDescriptions()
	vals, err := rows.Values()
	require.NoError(t, err)
	require.NoError(t, rows.Err())

	got := make(map[string]string, len(fds))
	for i, fd := range fds {
		got[fd.Name] = toStr(vals[i])
	}
	return got
}

func toStr(v any) string {
	if v == nil {
		return "<NULL>"
	}
	switch x := v.(type) {
	case time.Time:
		return x.Format(time.RFC3339Nano)
	case [16]byte:
		return (uuid.UUID)(x).String()
	case fmt.Stringer:
		return x.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}