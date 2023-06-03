package pgsnap

import (
	"context"
	"database/sql"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v4"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

const addr = "postgres://postgres@127.0.0.1:15432/?sslmode=disable"

func TestSnap_runScript_pq(t *testing.T) {
	db, s := NewDB(t, addr)
	defer s.Finish()

	runPQ(t, db)
}

func TestSnap_runScript_pgx(t *testing.T) {
	s := NewSnap(t, addr)
	defer s.Finish()

	runPGX(t, s.Addr())
}

func TestSnap_runProxy_pq(t *testing.T) {
	t.Skip("Still figure out how to design two connection")
	var s *Snap
	wg2 := sync.WaitGroup{}
	wg := sync.WaitGroup{}
	wg.Add(1)
	wg2.Add(1)
	go t.Run("inception", func(t *testing.T) {
		s = NewSnap(t, addr)
		wg.Done()
		wg2.Wait()
		defer s.Finish()
	})

	wg.Wait()

	db, s := NewDBWithConfig(t, s.Addr(), Config{ForceWrite: true})
	defer s.Finish()

	runPQ(t, db)
	wg2.Done()
}

func TestSnap_runProxy_pgx(t *testing.T) {
	t.Skip("try to use dockertest")
	s := NewSnapWithForceWrite(t, addr, true)
	defer s.Finish()

	runPGX(t, s.Addr())
}

func TestSnap_runEmptyScript(t *testing.T) {
	t.Skip("try to use dockertest")
	db, s := NewDB(t, addr)
	defer s.Finish()

	runPQ(t, db)

	// revert to empty file again
	script := newScript(t)
	_ = os.WriteFile(script.getFilename(), []byte(""), os.ModePerm)
}

func runPQ(t *testing.T, db *sql.DB) {
	t.Helper()

	err := db.Ping()
	require.NoError(t, err)

	rows, err := db.Query("select id from mytable limit $1", 7)
	require.NoError(t, err)

	_ = rows.Close()
}

func runPGX(t *testing.T, addr string) {
	t.Helper()

	db, err := pgx.Connect(context.TODO(), addr)
	require.NoError(t, err)

	err = db.Ping(context.TODO())
	require.NoError(t, err)

	_, err = db.Query(context.TODO(), "select id from mytable limit $1", 7)
	require.NoError(t, err)
}
