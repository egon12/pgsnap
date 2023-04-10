package pgsnap

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

const addrTmpl = "postgres://postgres@127.0.0.1:%s/?sslmode=disable"

var addr = "postgres://postgres@127.0.0.1:15432/?sslmode=disable"

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		os.Exit(m.Run())
	}

	var err error
	var finish func() error

	addr, finish, err = runPostgresInDocker()
	if err != nil {
		log.Fatal(err)
	}
	defer finish()

	code := m.Run()

	err = finish()
	if err != nil {
		log.Fatal(err)
	}

	os.Exit(code)
}

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
	if testing.Short() {
		t.Skip("skip need docker test")
	}

	db, s := NewDBWithConfig(t, addr, Config{ForceWrite: true})
	//defer db.Close()
	defer s.Finish()

	runPQ(t, db)
}

func TestSnap_runProxy_pgx(t *testing.T) {
	if testing.Short() {
		t.Skip("skip need docker test")
	}

	s := NewSnapWithForceWrite(t, addr, true)
	defer s.Finish()

	runPGX(t, s.Addr())
}

func TestSnap_runEmptyScript(t *testing.T) {
	if testing.Short() {
		t.Skip("Need dockertest")
	}
	db, s := NewDB(t, addr)
	//defer db.Close()
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

	//_ = db.Close(context.TODO())
}
