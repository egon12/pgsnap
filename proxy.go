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

func (s *Snap) runProxy(url string) {
	s.writeMode = true

	out, err := os.Create(s.getFilename())
	if err != nil {
		s.t.Fatalf("can't create file %s: %v", s.getFilename(), err)
	}

	db, err := pgx.Connect(context.TODO(), url)
	if err != nil {
		s.t.Fatalf("can't connect to db %s: %v", url, err)
	}

	go s.acceptConnForProxy(db, out)
}

func (s *Snap) acceptConnForProxy(db *pgx.Conn, out io.Writer) {
	conn, err := s.l.Accept()
	if err != nil {
		s.errchan <- err
		return
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

		if msg != nil {
			err = fe.Send(msg)
			if err != nil {
				s.t.Errorf("pgsnap: cannot forward to postgre: %T: %+v", msg, msg)
			}
		}
	}
}

func (s *Snap) streamFEtoBE(fe *pgproto3.Frontend, be *pgproto3.Backend, out io.Writer) {
	for {
		msg, err := fe.Receive()
		if err != nil {
			s.errchan <- err
			continue
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
	be.Send(&pgproto3.AuthenticationOk{})
	be.Send(&pgproto3.BackendKeyData{ProcessID: 0, SecretKey: 0})
	be.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})

	return be
}

func (s *Snap) prepareFrontend(db *pgx.Conn) *pgproto3.Frontend {
	conn := db.PgConn().Conn()
	return pgproto3.NewFrontend(pgproto3.NewChunkReader(conn), conn)
}
