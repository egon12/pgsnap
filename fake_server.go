package pgsnap

import (
	"log"
	"net"
	"time"

	"github.com/jackc/pgmock"
	"github.com/jackc/pgproto3/v2"
)

type (
	server struct {
		l       net.Listener
		errchan chan<- error
		done    chan<- struct{}
	}
)

// NewServer will create FakePostgresServer with errchan and donechan
func NewServer(l net.Listener, errchan chan<- error, done chan<- struct{}) *server {
	return &server{
		l:       l,
		errchan: errchan,
		done:    done,
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
		s.errchan <- err
		return
	}
	defer conn.Close()

	if err = conn.SetDeadline(time.Now().Add(time.Second)); err != nil {
		s.errchan <- err
		return
	}

	be := pgproto3.NewBackend(pgproto3.NewChunkReader(conn), conn)

	if err := script.Run(be); err != nil {
		s.waitTilSync(be)
		s.sendError(be, err)

		_ = conn.(*net.TCPConn).SetLinger(0)
		s.errchan <- err
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
		log.Printf("failed send be error (%s) caused by %s", err, postgresError)
	}

	// ignore the error
	_ = be.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
}
