package pgsnap

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgproto3/v2"
	"github.com/jackc/pgx/v4"
)

type proxy struct {
	t         testing.TB
	dsn       string
	script    *script
	l         net.Listener
	isDebug   bool
	done      atomic.Bool
	doneMutex sync.Mutex
}

func newProxy(t testing.TB, dsn string, script *script, l net.Listener, isDebug bool) *proxy {
	return &proxy{
		t:       t,
		dsn:     dsn,
		script:  script,
		l:       l,
		isDebug: isDebug,
		done:    atomic.Bool{},
	}
}

func (s *proxy) run() {
	s.t.Helper()
	outFilename := s.script.getFilename()

	out, err := os.Create(outFilename)
	if err != nil {
		s.t.Fatalf("can't create file %s: %v", outFilename, err)
	}

	db, err := pgx.Connect(context.TODO(), s.dsn)
	if err != nil {
		s.t.Fatalf("can't connect to db %s: %v", s.dsn, err)
	}

	err = db.Ping(context.TODO())
	if err != nil {
		s.t.Fatalf("can't ping to db %s: %v", s.dsn, err)
	}

	// only accept one connection / test. This is a limitation of the current
	// implementation.
	go s.acceptConnForProxy(db, out)
}

func (s *proxy) finish() {
	if s == nil {
		return
	}

	s.debugLogf("pgsnap: proxy finish")
	s.setDone()
}

func (s *proxy) acceptConnForProxy(db *pgx.Conn, out io.Writer) {
	conn, err := s.l.Accept()
	if err != nil {
		log.Println("server: cannot accept connection:", err)
		s.t.Errorf("server: cannot accept connection: %v", err)
		return
	}
	if s.isDebug {
		s.t.Log("accepting connection")
	}

	be := s.prepareBackend(conn)

	fe := s.prepareFrontend(db)

	s.runConversation(fe, be, out)
}

// runConversation will run conversation between frontend and backend
func (s *proxy) runConversation(fe *pgproto3.Frontend, be *pgproto3.Backend, out io.Writer) {
	go s.streamBEtoFE(fe, be, out)
	go s.streamFEtoBE(fe, be, out)
}

// streamBEtoFE streams messages from test to frontend
// this get message from test script and it will be saved to file
func (s *proxy) streamBEtoFE(fe *pgproto3.Frontend, be *pgproto3.Backend, out io.Writer) {
	for {
		s.debugLogf("pgsnap: BE receiving")
		msg, err := be.Receive()
		if err != nil {
			if s.isDone() {
				s.debugLogf("pgsnap: error on receive after test done: %v", err)
				return
			}
			if err == io.ErrUnexpectedEOF {
				s.t.Errorf("pgsnap: BE got unexpectedEOF. Maybe db client exited?")
				return
			}

			s.t.Errorf("pgsnap: BE failed receive message: %v", err)
			continue
		}

		b, err := json.Marshal(msg)
		if err != nil {
			s.t.Errorf("pgsnap: BE cannot marshal: %T: %+v", msg, msg)
		}
		if len(b) > 0 {
			b = append([]byte{'F', ' '}, b...)
			b = append(b, []byte("\n")...)
			_, _ = out.Write(b)
		}
		s.debugLogf("pgsnap: BE create FE obj %T: %+v", msg, msg)

		if msg != nil {
			s.debugLogf("pgsnap: BE send to database: %+v", msg)
			err = fe.Send(msg)
			s.debugLogf("pgsnap: BE send to database done err: %v", err)
			if err != nil {
				s.t.Errorf("pgsnap: BE cannot forward to postgre: %T: %+v", msg, msg)
			}
		}

		if s.isDone() {
			s.debugLogf("pgsnap: BE exit loop")
			return
		}
	}
}

func (s *proxy) streamFEtoBE(fe *pgproto3.Frontend, be *pgproto3.Backend, out io.Writer) {
	for {
		s.debugLogf("pgsnap: FE receiving")

		msg, err := fe.Receive()
		if err != nil {
			if s.isDone() {
				s.debugLogf("pgsnap: FE loop exit, error after done: %v", err)
				return
			}

			if err == io.ErrUnexpectedEOF {
				s.t.Errorf("pgsnap: unexpectedEOF from Database. Maybe database exit unexpectedly?")
				return
			}

			s.t.Errorf("pgsnap: error when FE receive: %v", err)
			continue
		}

		s.debugLogf("pgsnap: FE receive Database message %T: %+v", msg, msg)

		b, err := json.Marshal(msg)
		if err != nil {
			s.t.Errorf("pgsnap: FE cannot marshal Database message: %T: %+v", msg, msg)
		}
		if len(b) > 0 {
			b = append([]byte{'B', ' '}, b...)
			b = append(b, []byte("\n")...)
			_, _ = out.Write(b)
		}
		s.debugLogf("pgsnap: FE forward to test %T: %+v", msg, msg)

		if msg != nil {
			be.Send(msg)
			if err != nil {
				s.t.Errorf("pgsnap: FE forward to client error: %T: %+v :%v", msg, msg, err)
			}
		}

		if s.isDone() {
			s.debugLogf("pgsnap: FE exit loop")
			return
		}
	}
}

func (s *proxy) prepareBackend(conn net.Conn) *pgproto3.Backend {
	be := pgproto3.NewBackend(pgproto3.NewChunkReader(conn), conn)

	// expect startup message
	_, _ = be.ReceiveStartupMessage()
	_ = be.Send(&pgproto3.AuthenticationOk{})
	_ = be.Send(&pgproto3.BackendKeyData{ProcessID: 0, SecretKey: 0})
	_ = be.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})

	return be
}

func (s *proxy) prepareFrontend(db *pgx.Conn) *pgproto3.Frontend {
	conn := db.PgConn().Conn()
	return pgproto3.NewFrontend(pgproto3.NewChunkReader(conn), conn)
}

func (s *proxy) debugLogf(format string, args ...interface{}) {
	if s.isDebug {
		if !s.isDone() {
			s.t.Helper()
			s.t.Logf(format, args...)
			return
		}
		log.Printf(s.t.Name()+": "+format, args...)
	}
}

func (s *proxy) setDone() {
	s.doneMutex.Lock()
	defer s.doneMutex.Unlock()
	s.done.Store(true)
}

func (s *proxy) isDone() bool {
	s.doneMutex.Lock()
	defer s.doneMutex.Unlock()
	return s.done.Load()
}
