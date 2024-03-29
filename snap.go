package pgsnap

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"testing"
	"time"
)

type Snap struct {
	t       testing.TB
	addr    string
	msgchan chan string
	done    chan struct{}
	l       net.Listener
	isDebug bool

	proxy  *proxy  // will be fill if using proxy
	server *server // will be fill if using fake server

	finishFuncs []func() error
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
	t.Helper()
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
	t.Helper()
	cfg = setDefaultValue(cfg)

	s := &Snap{
		t:       t,
		msgchan: make(chan string, 100),
		done:    make(chan struct{}, 1),
		isDebug: cfg.Debug,
	}

	s.setFailAfter(cfg.TestTimeout)

	s.listen()

	script := newScript(t)

	if cfg.ForceWrite {
		s.runProxy(t, url, script, cfg)
		return s
	}

	pgxScript, err := script.Read()
	if s.shouldRunProxy(err) {
		s.runProxy(t, url, script, cfg)
		return s
	}

	if err != nil {
		s.t.Fatalf("can't open file \"%s\": %v", script.getFilename(), err)
	}

	s.server = newServer(s.l, s.done, s.t, s.isDebug)
	s.server.Run(pgxScript)

	return s
}

func (s *Snap) runProxy(t testing.TB, url string, script *script, cfg Config) {
	t.Helper()
	s.proxy = newProxy(t, url, script, s.l, cfg.Debug)
	s.proxy.run()
}

// setFaileAfter will call (*testing.T).Fatalf after timeout
func (s *Snap) setFailAfter(timeout time.Duration) {
	start := time.Now()
	go func() {
		select {
		case <-time.After(timeout):
			log.Printf("pgsnap timeout after %v, start at %v, end at %v", timeout, start, time.Now())
			s.t.Errorf("pgsnap timeout after %v", timeout)
			s.t.FailNow()
		case <-s.done:
		}
	}()
}

func (s *Snap) Finish() {
	// ignore the error
	_ = s.l.Close()

	if s.proxy != nil {
		s.proxy.finish()
		s.done <- struct{}{}
	}

	if s.server != nil {
		s.server.Wait()
	}

	for _, f := range s.finishFuncs {
		err := f()
		if err != nil {
			s.t.Error(err)
		}
	}
}

// AddFinishFunc will add function that will be called when
// Finish() is called. It used by docker to remove container
func (s *Snap) AddFinishFunc(f func() error) {
	s.finishFuncs = append(s.finishFuncs, f)
}

// Addr will return proxy / fake postgres address in form of
// postgres://user:password@127.0.0.1:15432/postgres
func (s *Snap) Addr() string {
	return s.addr
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
