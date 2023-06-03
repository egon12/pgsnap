package docker

import (
	"flag"
	"log"
	"os"
	"testing"
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

func RunPostgreInT(t *testing.T) (string, func() error, error) {
	if testing.Short() {
		t.Skip("skip need docker test")
	}

	p, err := NewPostgreInDocker(PostgresConfig{
		DebugMode:           false,
		ContainerNameSuffix: t.Name(),
	})
	return p.GetAddr(), func() error { return p.Finish() }, err
}
