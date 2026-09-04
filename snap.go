package pgsnap

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
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

	compMu sync.Mutex
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

	s.listen()

	s.setFailAfter(cfg.TestTimeout)

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

	srv := newServer(s.l, s.done, s.t, s.isDebug)
	s.compMu.Lock()
	s.server = srv
	s.compMu.Unlock()
	srv.Run(pgxScript)

	return s
}

func (s *Snap) runProxy(t testing.TB, url string, script *script, cfg Config) {
	t.Helper()
	p := newProxy(t, url, script, s.l, cfg.Debug)
	s.compMu.Lock()
	s.proxy = p
	s.compMu.Unlock()
	p.run()
}

// setFailAfter marks the test failed and force-closes any open connection
// after timeout, so a mismatched/hanging script doesn't block the test
// forever.
//
// This runs in its own goroutine, so it must not call (*testing.T).FailNow
// (or Fatal/Fatalf/Skip*): those call runtime.Goexit, which only unwinds
// this goroutine, not the goroutine actually running the test. If the test
// goroutine is blocked on a DB call to the fake server/proxy, FailNow here
// would never unblock it, and the test would hang until go test's own
// (much longer) global timeout instead of this one. So instead we mark the
// test failed with Errorf (safe from any goroutine) and force-close the
// underlying connections so the test goroutine's blocked read/write
// returns an error and the test can finish on its own.
func (s *Snap) setFailAfter(timeout time.Duration) {
	start := time.Now()
	go func() {
		select {
		case <-time.After(timeout):
			log.Printf("pgsnap timeout after %v, start at %v, end at %v", timeout, start, time.Now())
			s.t.Errorf("pgsnap timeout after %v", timeout)

			_ = s.l.Close()

			s.compMu.Lock()
			p, srv := s.proxy, s.server
			s.compMu.Unlock()

			if p != nil {
				p.Close()
			}
			if srv != nil {
				srv.Close()
			}
		case <-s.done:
		}
	}()
}

func (s *Snap) Finish() {
	// ignore the error
	_ = s.l.Close()

	s.compMu.Lock()
	p, srv := s.proxy, s.server
	s.compMu.Unlock()

	if p != nil {
		p.finish()
		s.done <- struct{}{}
	}

	if srv != nil {
		srv.Wait()
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
