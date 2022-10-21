package pgsnap

import (
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
	t         *testing.T
	addr      string
	errchan   chan error
	msgchan   chan string
	done      chan struct{}
	writeMode bool
	l         net.Listener
}

// NewSnap will create snap
func NewSnap(t *testing.T, postgreURL string) *Snap {
	return NewSnapWithForceWrite(t, postgreURL, false)
}

// NewSnapWithForceWrite function  
func NewSnapWithForceWrite(t *testing.T, url string, forceWrite bool) *Snap {
	s := &Snap{
		t:       t,
		errchan: make(chan error, 100),
		msgchan: make(chan string, 100),
		done:    make(chan struct{}, 1),
	}

	s.listen()

	script, err := s.getScript()
	if s.shouldRunProxy(forceWrite, err) {
		s.runProxy(url)
		return s
	}

	if err != nil {
		s.t.Fatalf("can't open file \"%s\": %v", s.getFilename(), err)
	}

	s.runFakePostgres(script)
	return s
}

func (s *Snap) Finish() {
	err := s.WaitFor(5 * time.Second)
	if err != nil {
		s.t.Helper()
		s.t.Error(err)
	}
}

func (s *Snap) Addr() string {
	return s.addr
}

func (s *Snap) WaitFor(d time.Duration) error {
	if s.writeMode {
		close(s.done)
	}

	select {
	case <-time.After(d):
		return errors.New("pgsnap timeout")
	case e := <-s.errchan:
		return e
	case <-s.done:
		return nil
	}
}

func (s *Snap) getFile() (*os.File, error) {
	return os.Open(s.getFilename())
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

func (s *Snap) shouldRunProxy(forceWrite bool, err error) bool {
	if forceWrite == true {
		return true
	}

	if os.IsNotExist(err) {
		return true
	}

	if errors.Is(EmptyScript, err) {
		return true
	}

	return false
}
