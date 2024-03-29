package pgsnap

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/stretchr/testify/assert"
)

// wrong query, should not cause a panic.
func Test_error_case(t *testing.T) {
	s := NewSnap(t, addr)
	defer s.Finish()

	ctx := context.Background()

	conn, _ := pgx.Connect(ctx, s.Addr())
	defer conn.Close(ctx)

	t.Run("non_existent_table", func(t *testing.T) {
		_, err := conn.Query(ctx, "SELECT * FROM non_existing_table WHERE id = $1", 1)

		assert.Equal(t, err, &pgconn.PgError{
			Severity:         "ERROR",
			Code:             "42P01",
			Message:          "relation \"non_existing_table\" does not exist",
			Position:         15,
			InternalPosition: 0,
			File:             "parse_relation.c",
			Line:             1376,
			Routine:          "parserOpenTable",
		})
	})

	t.Run("empty_rows", func(t *testing.T) {
		rows, err := conn.Query(ctx, "SELECT * FROM mytable WHERE id = $1", 4)
		assert.NoError(t, err)
		assert.False(t, rows.Next())
	})
}

func Test_if_not_accept_should_throw_timeout(t *testing.T) {
	tb := newFakeTB(t)

	s := NewSnapWithConfig(tb, addr, Config{
		TestTimeout: 10 * time.Millisecond,
	})
	defer s.Finish()

	time.Sleep(100 * time.Millisecond)

	assert.True(t, tb.FailNowCalled.Load())
}

type fakeTB struct {
	testing.TB
	ErrorMessages []string
	FailNowCalled *atomic.Bool
}

func newFakeTB(t testing.TB) *fakeTB {
	return &fakeTB{
		TB:            t,
		FailNowCalled: &atomic.Bool{},
	}
}

func (f *fakeTB) Error(args ...interface{}) {
	f.ErrorMessages = append(f.ErrorMessages, fmt.Sprintln(args...))
}

func (f *fakeTB) Errorf(format string, args ...interface{}) {
	f.ErrorMessages = append(f.ErrorMessages, fmt.Sprintf(format, args...))
}

func (f *fakeTB) FailNow() {
	f.FailNowCalled.Store(true)
}
