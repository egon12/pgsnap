package pgsnap

import (
	"context"
	"testing"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/stretchr/testify/assert"
)

// wrong query, should not cause a panic.
func Test_error_case(t *testing.T) {
	s := NewSnap(t, addr)

	ctx := context.Background()

	conn, _ := pgx.Connect(ctx, s.Addr())

	t.Run("non_existent_table", func(t *testing.T) {
		_, err := conn.Query(ctx, "SELECT * FROM non_existing_table WHERE id = $1", 1)

		assert.Equal(t, err, &pgconn.PgError{
			Severity:         "ERROR",
			Code:             "42P01",
			Message:          "relation \"non_existing_table\" does not exist",
			Position:         15,
			InternalPosition: 0,
			File:             "parse_relation.c",
			Line:             1384,
			Routine:          "parserOpenTable",
		})
	})

	t.Run("empty_rows", func(t *testing.T) {
		rows, err := conn.Query(ctx, "SELECT * FROM mytable WHERE id = $1", 4)
		assert.NoError(t, err)
		assert.False(t, rows.Next())
	})
}
