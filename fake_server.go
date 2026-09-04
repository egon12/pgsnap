package pgsnap

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgmock"
	"github.com/jackc/pgproto3/v2"
)

type (
	server struct {
		t       testing.TB
		l       net.Listener
		done    chan<- struct{}
		isDebug bool
		wg      sync.WaitGroup

		connMu sync.Mutex
		conn   net.Conn
	}
)

// newServer will create FakePostgresServer with errchan and donechan
func newServer(l net.Listener,
	done chan<- struct{},
	t testing.TB,
	isDebug bool,
) *server {
	return &server{
		l:       l,
		done:    done,
		t:       t,
		isDebug: isDebug,
	}
}

// Run will
func (s *server) Run(script *pgmock.Script) {
	s.runFakePostgres(script)
}

func (s *server) Wait() {
	s.wg.Wait()
}

func (s *server) runFakePostgres(script *pgmock.Script) {
	s.wg.Add(1)
	go s.acceptConnForScript(script)
}

func (s *server) acceptConnForScript(script *pgmock.Script) {
	// need to defer this to make sure we send the done signal
	defer func() {
		s.debugLogf("server: finish script")
		s.wg.Done()
		s.done <- struct{}{}
	}()

	conn, err := s.l.Accept()
	if err != nil {
		s.t.Errorf("server: cannot accept connection: %v", err)
		return
	}
	defer conn.Close()
	s.setConn(conn)
	s.debugLogf("server: accepted connection")

	be := pgproto3.NewBackend(pgproto3.NewChunkReader(conn), conn)

	s.debugLogf("server: run script")
	if err := script.Run(be); err != nil {
		s.t.Errorf("server: run script got error: %v", err)
		s.waitTilSync(be)
		s.sendError(be, err)
		return
	}
}

func (s *server) waitTilSync(be *pgproto3.Backend) {
	for i := 0; i < 10; i++ {
		msg, err := be.Receive()
		if err != nil {
			continue
		}

		_, ok := msg.(*pgproto3.Sync)
		if ok {
			break
		}
	}
}

func (s *server) sendError(be *pgproto3.Backend, postgresError error) {
	err := be.Send(&pgproto3.ErrorResponse{
		Severity:            "ERROR",
		SeverityUnlocalized: "ERROR",
		Code:                "99999",
		Message:             "pgsnap:\n" + postgresError.Error(),
	})
	if err != nil {
		s.t.Errorf("BE send Error (%s) caused by %s", err, postgresError)
	}

	// ignore the error
	_ = be.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
}

func (s *server) setConn(conn net.Conn) {
	s.connMu.Lock()
	defer s.connMu.Unlock()
	s.conn = conn
}

// Close forcibly closes the active connection to unblock any goroutine
// (including the test's own) that is blocked on a read/write to it. Used
// when the test exceeds its timeout, since a mismatched script otherwise
// leaves the client waiting for a response that will never come.
func (s *server) Close() {
	s.connMu.Lock()
	defer s.connMu.Unlock()
	if s.conn != nil {
		_ = s.conn.Close()
	}
}

func (s *server) debugLogf(format string, args ...interface{}) {
	if s.isDebug {
		s.t.Helper()
		args = append([]interface{}{time.Now()}, args...)
		s.t.Logf("%v: "+format, args...)
	}
}
