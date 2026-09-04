package pgsnap

import (
	"context"
	"fmt"
	"sync"
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

// setFailAfter must not call (*testing.T).FailNow from its own goroutine:
// FailNow only unwinds the goroutine that calls it (via runtime.Goexit), not
// the goroutine actually running the test. If the test goroutine is blocked
// on a DB call to the fake server/proxy, FailNow from setFailAfter's
// goroutine would never unblock it, leaving the test hanging until go test's
// own (much longer) global timeout. So on timeout it must instead mark the
// test failed with Errorf (safe from any goroutine) and force-close the
// underlying listener so a blocked test goroutine gets unblocked and the
// test can finish on its own.
func Test_if_not_accept_should_throw_timeout(t *testing.T) {
	tb := newFakeTB(t)

	s := NewSnapWithConfig(tb, addr, Config{
		TestTimeout: 10 * time.Millisecond,
	})
	defer s.Finish()

	time.Sleep(100 * time.Millisecond)

	assert.False(t, tb.FailNowCalled.Load())
	assert.NotEmpty(t, tb.errorMessages())
}

type fakeTB struct {
	testing.TB
	ErrorMessages []string
	FailNowCalled *atomic.Bool
	mu            sync.Mutex
}

func newFakeTB(t testing.TB) *fakeTB {
	return &fakeTB{
		TB:            t,
		FailNowCalled: &atomic.Bool{},
	}
}

func (f *fakeTB) Error(args ...interface{}) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ErrorMessages = append(f.ErrorMessages, fmt.Sprintln(args...))
}

func (f *fakeTB) Errorf(format string, args ...interface{}) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ErrorMessages = append(f.ErrorMessages, fmt.Sprintf(format, args...))
}

func (f *fakeTB) FailNow() {
	f.FailNowCalled.Store(true)
}

func (f *fakeTB) errorMessages() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ErrorMessages
}
