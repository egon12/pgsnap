package pgsnap

import (
	"errors"
	"fmt"
	"net"
	"os"
	"testing"
	"time"
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

func NewSnap(t *testing.T, postgreURL string) *Snap {
	return NewSnapWithForceWrite(t, postgreURL, false)
}

// NewSnap
func NewSnapWithForceWrite(t *testing.T, url string, forceWrite bool) *Snap {
	s := &Snap{
		t:       t,
		errchan: make(chan error, 100),
		msgchan: make(chan string, 100),
		done:    make(chan struct{}, 1),
	}

	s.listen()

	f, err := s.getFile()
	if s.shouldRunProxy(forceWrite, err) {
		s.runProxy(url)
		return s
	}

	if err != nil {
		s.t.Fatalf("can't open file \"%s\": %v", s.getFilename(), err)
	}

	s.runScript(f)
	return s
}

func (s *Snap) Finish() {
	err := s.Wait()
	if err != nil {
		s.t.Helper()
		s.t.Error(err)
	}
}

func (s *Snap) Addr() string {
	return s.addr
}

func (s *Snap) Wait() error {
	return s.WaitFor(5 * time.Second)
}

func (s *Snap) WaitFor(d time.Duration) error {
	if s.writeMode {
		s.done <- struct{}{}
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
	return s.t.Name() + ".txt"
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

	return false
}
