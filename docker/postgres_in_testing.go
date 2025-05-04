package docker

import (
	"flag"
	"log"
	"os"
	"testing"

	"github.com/egon12/pgsnap"
)

var addrInM string

func GetAddr() string {
	return addrInM
}

func RunPostgreInM(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		os.Exit(m.Run())
	}

	p, err := NewPostgreInDocker(PostgresConfig{DebugMode: false})
	if err != nil {
		log.Fatal(err)
	}
	defer p.Finish()
	addrInM = p.GetAddr()

	code := m.Run()

	err = p.Finish()
	if err != nil {
		log.Fatal(err)
	}

	os.Exit(code)
}

func RunPostgreInT(t *testing.T, options ...Options) (string, func() error, error) {
	if testing.Short() {
		t.Skip("skip need docker test")
	}

	p, err := NewPostgreInDocker(PostgresConfig{
		DebugMode:           false,
		ContainerNameSuffix: t.Name(),
	})
	return p.GetAddr(), p.Finish, err
}

func NewSnapWithDocker(t *testing.T, options ...Options) (*pgsnap.Snap, error) {
	t.Helper()

	var addr string
	var finish func() error

	var cfg Config
	cfg.ContainerNameSuffix = t.Name()
	cfg.KeepContainer = true

	for _, o := range options {
		o(&cfg)
	}

	if cfg.ForceWrite || !pgsnap.IsSnapshotExists(t) {
		if testing.Short() {
			t.Skip("skip need docker test")
		}

		p, err := NewPostgreInDocker(cfg.PostgresConfig)
		if err != nil {
			t.Fatalf("docker failed: %v", err)
		}
		addr = p.GetAddr()
		finish = p.Finish
	}

	snap := pgsnap.NewSnap(t, addr)

	if finish != nil {
		snap.AddFinishFunc(finish)
	}

	return snap, nil
}

func NewPgSnapDocker(t *testing.T, options ...Options) *pgsnap.Snap {
	snap, err := NewSnapWithDocker(t, options...)
	if err != nil {
		t.Fatal(err)
		return nil
	}
	return snap
}
