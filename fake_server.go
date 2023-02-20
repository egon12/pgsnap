package pgsnap

import (
	"net"
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

func (s *server) runFakePostgres(script *pgmock.Script) {
	go s.acceptConnForScript(script)
}

func (s *server) acceptConnForScript(script *pgmock.Script) {
	conn, err := s.l.Accept()
	if err != nil {
		s.t.Errorf("server: cannot accept connection: %v", err)
		return
	}
	defer conn.Close()
	s.debugLogf("server: accepted connection")

	if err = conn.SetDeadline(time.Now().Add(time.Second)); err != nil {
		s.t.Errorf("server: cannott set deadline: %v", err)
		return
	}

	be := pgproto3.NewBackend(pgproto3.NewChunkReader(conn), conn)

	s.debugLogf("server: run script")
	if err := script.Run(be); err != nil {
		s.t.Errorf("server: run script got error: %v", err)
		s.waitTilSync(be)
		s.sendError(be, err)

		_ = conn.(*net.TCPConn).SetLinger(0)
		return
	}

	s.done <- struct{}{}
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

func (s *server) debugLogf(format string, args ...interface{}) {
	if s.isDebug {
		s.t.Logf(format, args...)
	}
}
