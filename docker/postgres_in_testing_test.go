package docker

import (
	"context"
	"database/sql"
	"testing"

	"github.com/egon12/pgsnap"
	"github.com/jackc/pgx/v5"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestSnap_runScript_pq(t *testing.T) {
	addr, finish, err := RunPostgreInT(t)
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	db, s := pgsnap.NewDB(t, addr)
	defer s.Finish()

	runPQ(t, db)
}

func TestSnap_runScript_pgx(t *testing.T) {
	addr, finish, err := RunPostgreInT(t)
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	s := pgsnap.NewSnap(t, addr)
	defer s.Finish()

	runPGX(t, s.Addr())
}

func TestSnap_runProxy_pq(t *testing.T) {
	if testing.Short() {
		t.Skip("skip need docker test")
	}

	addr, finish, err := RunPostgreInT(t)
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	db, s := pgsnap.NewDBWithConfig(t, addr, pgsnap.Config{ForceWrite: true})
	defer db.Close()
	defer s.Finish()

	runPQ(t, db)
}

func TestSnap_runProxy_pgx(t *testing.T) {
	if testing.Short() {
		t.Skip("skip need docker test")
	}

	addr, finish, _ := RunPostgreInT(t)
	defer finish()
	s := pgsnap.NewSnapWithForceWrite(t, addr, true)
	defer s.Finish()

	runPGX(t, s.Addr())
}

func TestSnap_runEmptyScript(t *testing.T) {
	if testing.Short() {
		t.Skip("Need dockertest")
	}

	addr, finish, err := RunPostgreInT(t)
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	db, s := pgsnap.NewDB(t, addr)
	defer s.Finish()
	defer db.Close()

	runPQ(t, db)

	// revert to empty file again
	// script := newScript(t)
	// _ = os.WriteFile(script.getFilename(), []byte(""), os.ModePerm)
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

	_ = db.Close(context.TODO())
}
