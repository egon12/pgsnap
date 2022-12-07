package pgsnap

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/jackc/pgmock"
	"github.com/jackc/pgproto3/v2"
)

type (
	script struct {
		t    testing.TB
		path string
	}
)

var EmptyScript = errors.New("script is empty")

func NewScript(t testing.TB, path string) *script {
	return &script{t: t, path: path}
}

func (s *script) ReadOnlyFile() (*os.File, error) {
	return os.OpenFile(s.path, os.O_RDONLY, 0)
}

func (s *script) Read() (*pgmock.Script, error) {
	f, err := s.ReadOnlyFile()
	if err != nil {
		return nil, err
	}
	defer f.Close()

	script := s.readScript(f)
	if len(script.Steps) < len(pgmock.AcceptUnauthenticatedConnRequestSteps())+1 {
		return script, EmptyScript
	}

	return script, nil
}

func (s *script) readScript(f io.Reader) *pgmock.Script {
	script := &pgmock.Script{
		Steps: pgmock.AcceptUnauthenticatedConnRequestSteps(),
	}

	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		b := scanner.Bytes()

		if len(b) < 2 {
			continue
		}

		switch b[0] {
		case 'B':
			msg := s.unmarshalB(b[1:])
			script.Steps = append(script.Steps, pgmock.SendMessage(msg))
		case 'F':
			msg := s.unmarshalF(b[1:])

			switch m := msg.(type) {
			case *pgproto3.Parse:
				script.Steps = append(script.Steps, &expectParseMessage{want: m})
			case *pgproto3.Describe:
				script.Steps = append(script.Steps, &expectDescribeMessage{want: m})
			case *pgproto3.Bind:
				script.Steps = append(script.Steps, &expectBindMessage{want: m})
			default:
				script.Steps = append(script.Steps, pgmock.ExpectMessage(m))
			}

		}
	}

	return script
}

func (s *script) unmarshalB(src []byte) pgproto3.BackendMessage {
	t := struct {
		Type string
	}{}

	if err := json.Unmarshal(src, &t); err != nil {
		s.t.Fatalf("unmarshal backend message failed: %v\nsource: %s", err, string(src))
		return nil
	}

	var o pgproto3.BackendMessage

	switch t.Type {
	case "AuthenticationOK":
		o = &pgproto3.AuthenticationOk{}
	case "BackendKeyData":
		o = &pgproto3.BackendKeyData{}
	case "ParseComplete":
		o = &pgproto3.ParseComplete{}
	case "ParameterStatus":
		o = &pgproto3.ParameterStatus{}
	case "ParameterDescription":
		o = &pgproto3.ParameterDescription{}
	case "RowDescription":
		o = &pgproto3.RowDescription{}
	case "ReadyForQuery":
		o = &pgproto3.ReadyForQuery{}
	case "BindComplete":
		o = &pgproto3.BindComplete{}
	case "DataRow":
		o = &pgproto3.DataRow{}
	case "CommandComplete":
		o = &pgproto3.CommandComplete{}
	case "EmptyQueryResponse":
		o = &pgproto3.EmptyQueryResponse{}
	case "NoData":
		o = &pgproto3.NoData{}
	case "ErrorResponse":
		o = &pgproto3.ErrorResponse{}
	case "CloseComplete":
		o = &pgproto3.CloseComplete{}
	case "CopyBothResponse":
		o = &pgproto3.CopyBothResponse{}
	case "CopyData":
		o = &pgproto3.CopyData{}
	case "CopyInResponse":
		o = &pgproto3.CopyInResponse{}
	case "CopyOutResponse":
		o = &pgproto3.CopyOutResponse{}
	case "CopyDone":
		o = &pgproto3.CopyDone{}
	case "FunctionCallResponse":
		o = &pgproto3.FunctionCallResponse{}
	case "NoticeResponse":
		o = &pgproto3.NoticeResponse{}
	case "NotificationResponse":
		o = &pgproto3.NotificationResponse{}
	case "PortalSuspended":
		o = &pgproto3.PortalSuspended{}

	default:
		s.t.Fatalf("unknown backend type: " + t.Type)
		return nil
	}

	if err := json.Unmarshal(src, o); err != nil {
		s.t.Fatalf("unmarshal backend message to %T failed\nsource: %s", o, string(src))
		return nil
	}

	return o
}

func (s *script) unmarshalF(src []byte) pgproto3.FrontendMessage {
	t := struct {
		Type string
	}{}

	if err := json.Unmarshal(src, &t); err != nil {
		s.t.Fatalf("unmarshal frontend message failed: %v\nsource: %s", err, string(src))
	}

	var o pgproto3.FrontendMessage

	switch t.Type {
	case "StartupMessage":
		o = &pgproto3.StartupMessage{}
	case "Parse":
		o = &pgproto3.Parse{}
	case "Query":
		o = &pgproto3.Query{}
	case "Describe":
		o = &pgproto3.Describe{}
	case "Sync":
		o = &pgproto3.Sync{}
	case "Bind":
		o = &pgproto3.Bind{}
	case "Execute":
		o = &pgproto3.Execute{}
	case "Terminate":
		o = &pgproto3.Terminate{}
	case "Close":
		o = &pgproto3.Close{}
	case "Flush":
		o = &pgproto3.Flush{}
	case "CopyData":
		o = &pgproto3.CopyData{}
	case "CopyDone":
		o = &pgproto3.CopyDone{}
	case "CopyFail":
		o = &pgproto3.CopyFail{}
	case "CancelRequest":
		o = &pgproto3.CancelRequest{}
	default:
		//
		// gssEncRequest  GSSEncRequest
		// sslRequest     SSLRequest
		panic("unknown type: " + t.Type)
	}

	if err := json.Unmarshal(src, o); err != nil {
		s.t.Fatalf("unmarshal backend message to %T failed\nsource: %s", o, string(src))
		return nil
	}

	return o
}
