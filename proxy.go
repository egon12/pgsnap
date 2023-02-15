package pgsnap

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"os"

	"github.com/jackc/pgproto3/v2"
	"github.com/jackc/pgx/v4"
)

func (s *Snap) runProxy(url string, outFilename string) {
	out, err := os.Create(outFilename)
	if err != nil {
		s.t.Fatalf("can't create file %s: %v", outFilename, err)
	}

	db, err := pgx.Connect(context.TODO(), url)
	if err != nil {
		s.t.Fatalf("can't connect to db %s: %v", url, err)
	}
	err = db.Ping(context.TODO())
	if err != nil {
		s.t.Fatalf("can't pint to db %s: %v", url, err)
	}

	go func() {
		for {
			s.acceptConnForProxy(db, out)
		}
	}()
}

func (s *Snap) acceptConnForProxy(db *pgx.Conn, out io.Writer) {
	conn, err := s.l.Accept()
	if err != nil {
		s.errchan <- err
		return
	}
	if s.isDebug {
		s.t.Log("accepting connection")
	}

	be := s.prepareBackend(conn)

	fe := s.prepareFrontend(db)

	s.runConversation(fe, be, out)
}

func (s *Snap) runConversation(fe *pgproto3.Frontend, be *pgproto3.Backend, out io.Writer) {
	go s.streamBEtoFE(fe, be, out)
	go s.streamFEtoBE(fe, be, out)
}

func (s *Snap) streamBEtoFE(fe *pgproto3.Frontend, be *pgproto3.Backend, out io.Writer) {
	for {
		msg, err := be.Receive()
		if err != nil {
			s.t.Errorf("pgsnap: BE cannot receive")
			s.errchan <- err
			continue
		}

		b, err := json.Marshal(msg)
		if err != nil {
			s.t.Errorf("pgsnap: cannot marshal: %T: %+v", msg, msg)
		}
		if len(b) > 0 {
			b = append([]byte{'F', ' '}, b...)
			b = append(b, []byte("\n")...)
			_, _ = out.Write(b)
		}
		if s.isDebug {
			s.t.Logf("pgsnap: %T: %+v", msg, msg)
		}

		if msg != nil {
			if s.isDebug {
				s.t.Logf("pgsnap: sending message: %+v", msg)
			}
			err = fe.Send(msg)
			if s.isDebug {
				s.t.Logf("pgsnap: sending message done err: %v", err)
			}
			if err != nil {
				s.t.Errorf("pgsnap: cannot forward to postgre: %T: %+v", msg, msg)
			}
		}
		if s.isDebug {
			s.t.Log("pgsnap: fe receiving2")
		}
	}
}

func (s *Snap) streamFEtoBE(fe *pgproto3.Frontend, be *pgproto3.Backend, out io.Writer) {
	for {
		if s.isDebug {
			s.t.Log("pgsnap: fe receiving")
		}
		msg, err := fe.Receive()
		if err != nil {
			s.t.Errorf("pgsnap: FE cannot receive")
			s.errchan <- err
			continue
		}
		if s.isDebug {
			s.t.Logf("pgsnap: message receive%T: %+v", msg, msg)
		}

		b, err := json.Marshal(msg)
		if err != nil {
			s.t.Errorf("pgsnap: cannot marshal: %T: %+v", msg, msg)
		}
		if len(b) > 0 {
			b = append([]byte{'B', ' '}, b...)
			b = append(b, []byte("\n")...)
			_, _ = out.Write(b)
		}
		if s.isDebug {
			s.t.Logf("pgsnap: %T: %+v", msg, msg)
		}

		if msg != nil {
			be.Send(msg)
			if err != nil {
				s.t.Errorf("pgsnap: cannot forward to client: %T: %+v", msg, msg)
			}
		}
	}
}

func (s *Snap) prepareBackend(conn net.Conn) *pgproto3.Backend {
	be := pgproto3.NewBackend(pgproto3.NewChunkReader(conn), conn)

	// expect startup message
	_, _ = be.ReceiveStartupMessage()
	_ = be.Send(&pgproto3.AuthenticationOk{})
	_ = be.Send(&pgproto3.BackendKeyData{ProcessID: 0, SecretKey: 0})
	_ = be.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})

	return be
}

func (s *Snap) prepareFrontend(db *pgx.Conn) *pgproto3.Frontend {
	conn := db.PgConn().Conn()
	return pgproto3.NewFrontend(pgproto3.NewChunkReader(conn), conn)
}
