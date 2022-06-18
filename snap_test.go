package pgsnap

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/jackc/pgx/v4"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var addr = "postgres://postgres@127.0.0.1:15432/?sslmode=disable"

func init() {
	// set PGSNAPURL to override the postgres test address
	if a := os.Getenv("PGSNAPURL"); a != "" {
		addr = a
	}
}

func TestSnap_runScript_pq(t *testing.T) {
	s := NewSnap(t, addr)
	defer s.Finish()

	runPQ(t, s.Addr())
}

func TestSnap_runScript_pgx(t *testing.T) {
	s := NewSnap(t, addr)
	defer s.Finish()

	runPGX(t, s.Addr())
}

func TestSnap_runProxy_pq(t *testing.T) {
	s := NewSnapWithForceWrite(t, addr, true)
	defer s.Finish()

	runPQ(t, s.Addr())
}

func TestSnap_runEmptyScript(t *testing.T) {
	s := NewSnap(t, addr)
	defer s.Finish()

	runPQ(t, s.Addr())

	// revert to empty file again
	os.WriteFile("TestSnap_runEmptyScript.txt", []byte(""), os.ModePerm)
}

func runPQ(t *testing.T, addr string) {
	t.Helper()

	db, err := sql.Open("postgres", addr)
	require.NoError(t, err)

	err = db.Ping()
	require.NoError(t, err)

	rows, err := db.Query("select id from mytable limit $1", 7)
	require.NoError(t, err)

	rows.Close()
}

func runPGX(t *testing.T, addr string) {
	t.Helper()

	db, err := pgx.Connect(context.TODO(), addr)
	require.NoError(t, err)

	err = db.Ping(context.TODO())
	require.NoError(t, err)

	_, err = db.Query(context.TODO(), "select id from mytable limit  $1", 7)
	require.NoError(t, err)
}

func Test_getFilename(t *testing.T) {
	s := &Snap{t: t}
	assert.Equal(t, "Test_getFilename.txt", s.getFilename())

	t.Run("another test name", func(t *testing.T) {
		s = &Snap{t: t}
		assert.Equal(t, "Test_getFilename/another_test_name.txt", s.getFilename())
	})

	t.Run("what about this one?", func(t *testing.T) {
		s = &Snap{t: t}
		assert.Equal(t, "Test_getFilename/what_about_this_one?.txt", s.getFilename())
	})
}
