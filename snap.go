package pgsnap

import (
	"database/sql"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"
	"unicode"
)

type Snap struct {
	t       testing.TB
	addr    string
	errchan chan error
	msgchan chan string
	done    chan struct{}
	l       net.Listener
	isDebug bool
}

type Config struct {
	// TestTimeout Default 5s
	TestTimeout time.Duration

	// Force to create proxy and connect to real postgres server
	ForceWrite bool

	// Debug if true it will print more verbose
	Debug bool
}

// NewDB will create *sql.DB to be used in the test
func NewDB(t testing.TB, url string) (*sql.DB, *Snap) {
	snap := NewSnap(t, url)
	db, err := sql.Open("postgres", snap.Addr())
	if err != nil {
		t.Fatal(err)
	}
	return db, snap
}

// NewDBWithConfig will create *sql.DB to be used in the test
// but it will ignore the snapshot file
func NewDBWithConfig(t testing.TB, url string, cfg Config) (*sql.DB, *Snap) {
	snap := NewSnapWithConfig(t, url, cfg)
	db, err := sql.Open("postgres", snap.Addr())
	if err != nil {
		t.Fatal(err)
	}
	return db, snap
}

// NewSnap will create snap
func NewSnap(t testing.TB, postgreURL string) *Snap {
	return NewSnapWithConfig(t, postgreURL, Config{
		ForceWrite:  os.Getenv("PGSNAP_FORCE_WRITE") == "true",
		Debug:       os.Getenv("PGSNAP_DEBUG") == "true",
		TestTimeout: 5 * time.Second,
	})
}

// Deprecated
// NewSnapWithForceWrite function
func NewSnapWithForceWrite(t testing.TB, url string, forceWrite bool) *Snap {
	return NewSnapWithConfig(t, url, Config{
		ForceWrite:  forceWrite,
		Debug:       os.Getenv("PGSNAP_DEBUG") == "true",
		TestTimeout: 5 * time.Second,
	})
}

// Make it private first, because we still design the api first
func NewSnapWithConfig(t testing.TB, url string, cfg Config) *Snap {
	cfg = setDefaultValue(cfg)

	s := &Snap{
		t:       t,
		errchan: make(chan error, 100),
		msgchan: make(chan string, 100),
		done:    make(chan struct{}, 1),
		isDebug: cfg.Debug,
	}

	s.setFailAfter(cfg.TestTimeout)

	s.listen()

	if cfg.ForceWrite {
		s.runProxy(url)
		return s
	}

	script := NewScript(t, s.getFilename())

	pgxScript, err := script.Read()
	if s.shouldRunProxy(err) {
		s.runProxy(url)
		return s
	}

	if err != nil {
		s.t.Fatalf("can't open file \"%s\": %v", s.getFilename(), err)
	}

	server := NewServer(s.l, s.errchan, s.done)
	server.Run(pgxScript)

	return s
}

// setFaileAfter will call (*testing.T).Fatalf after timeout
func (s *Snap) setFailAfter(timeout time.Duration) {
	go func() {
		select {
		case <-time.After(timeout):
			s.t.Errorf("pgsnap timeout after %v", timeout)
			s.t.FailNow()
		case <-s.done:
		}
	}()
}

func (s *Snap) Finish() {
	// ignore the error
	_ = s.l.Close()
}

// Addr will return proxy / fake postgres address in form of
// postgres://user:password@127.0.0.1:15432/postgres
func (s *Snap) Addr() string {
	return s.addr
}

func (s *Snap) getFilename() string {
	n := s.t.Name()
	n = strings.TrimPrefix(n, "Test")
	n = strings.ReplaceAll(n, "/", "__")
	n = strings.Map(func(r rune) rune {
		switch {
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			return r
		default:
			return '_'
		}
	}, n)
	n = strings.ToLower(n)
	return "pgsnap_" + n + ".txt"
}

func (s *Snap) listen() net.Listener {
	var err error

	s.l, err = net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		s.t.Fatal("can't open port: " + err.Error())
	}

	s.addr = fmt.Sprintf("postgres://user@%s/?sslmode=disable", s.l.Addr())

	return s.l
}

func (s *Snap) shouldRunProxy(err error) bool {
	if os.IsNotExist(err) {
		return true
	}

	if errors.Is(EmptyScript, err) {
		return true
	}

	return false
}

func setDefaultValue(cfg Config) Config {
	if cfg.TestTimeout == 0 {
		cfg.TestTimeout = 5 * time.Second
	}

	return cfg
}
