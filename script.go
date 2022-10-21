package pgsnap

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"unicode"

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

func NewScript(t testing.TB) *script {
	return &script{t: t}
}

// Path is the path to the script file.
func (s *script) Path() string {
	if s.path == "" {
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

		s.path = "pgsnap_" + n + ".txt"
	}
	return s.path
}

func (s *script) ReadOnlyFile() (*os.File, error) {
	return os.OpenFile(s.Path(), os.O_RDONLY, 0)
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

func (s *script) readScript(f *os.File) *pgmock.Script {
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

	_ = json.Unmarshal(src, &t)

	var o pgproto3.BackendMessage

	switch t.Type {
	case "AuthenticationOK":
		o = &pgproto3.AuthenticationOk{}
	case "BackendKeyData":
		o = &pgproto3.BackendKeyData{}
	case "ParseComplete":
		o = &pgproto3.ParseComplete{}
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
	default:
		panic("unknown type: " + t.Type)
	}

	_ = json.Unmarshal(src, o)

	return o
}

func (s *script) unmarshalF(src []byte) pgproto3.FrontendMessage {
	t := struct {
		Type string
	}{}

	json.Unmarshal(src, &t)

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
	default:
		panic("unknown type: " + t.Type)
	}

	_ = json.Unmarshal(src, o)

	return o
}
