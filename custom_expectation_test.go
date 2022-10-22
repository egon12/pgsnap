package pgsnap

import (
	"errors"
	"testing"

	"github.com/jackc/pgproto3/v2"
	"github.com/stretchr/testify/assert"
)

func Test_expectParseMessage_compare(t *testing.T) {
	tests := []struct {
		name    string
		field   *pgproto3.Parse
		arg     pgproto3.FrontendMessage
		wantErr error
	}{
		{
			name:    "success",
			field:   &pgproto3.Parse{},
			arg:     &pgproto3.Parse{},
			wantErr: nil,
		},
		{
			name:    "success empty",
			field:   &pgproto3.Parse{Name: "test", Query: "select * from test"},
			arg:     &pgproto3.Parse{Name: "test", Query: "select * from test"},
			wantErr: nil,
		},
		{
			name:    "success with different name",
			field:   &pgproto3.Parse{Name: "test", Query: "select * from test"},
			arg:     &pgproto3.Parse{Name: "another", Query: "select * from test"},
			wantErr: nil,
		},
		{
			name:    "different in type",
			field:   &pgproto3.Parse{},
			arg:     &pgproto3.Describe{},
			wantErr: errors.New("msg => *pgproto3.Describe, want => *pgproto3.Parse"),
		},
		{
			name:    "different in query",
			field:   &pgproto3.Parse{Name: "test", Query: "select * from test"},
			arg:     &pgproto3.Parse{Name: "another", Query: "select * from test1"},
			wantErr: errors.New("msg => query: select * from test1, want => query: select * from test"),
		},
		{
			name:    "different in ParameterOIDs",
			field:   &pgproto3.Parse{ParameterOIDs: []uint32{1, 2, 3}},
			arg:     &pgproto3.Parse{ParameterOIDs: []uint32{1, 2, 4}},
			wantErr: errors.New("msg => ParameterOIDs: [1 2 4], want => ParameterOIDs: [1 2 3]"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &expectParseMessage{want: tt.field}
			err := e.compare(tt.arg)
			assert.Equal(t, tt.wantErr, err)
		})
	}
}

func Test_expectDescribeMessage_compare(t *testing.T) {
	tests := []struct {
		name    string
		field   *pgproto3.Describe
		arg     pgproto3.FrontendMessage
		wantErr error
	}{
		{
			name:    "success",
			field:   &pgproto3.Describe{Name: "test", ObjectType: 'S'},
			arg:     &pgproto3.Describe{Name: "test", ObjectType: 'S'},
			wantErr: nil,
		},
		{
			name:    "success with different name",
			field:   &pgproto3.Describe{Name: "test", ObjectType: 'S'},
			arg:     &pgproto3.Describe{Name: "ping", ObjectType: 'S'},
			wantErr: nil,
		},
		{
			name:    "different in type",
			field:   &pgproto3.Describe{Name: "test"},
			arg:     &pgproto3.Parse{Name: "test"},
			wantErr: errors.New("msg => *pgproto3.Parse, want => *pgproto3.Describe"),
		},
		{
			name:    "different in object type",
			field:   &pgproto3.Describe{Name: "test", ObjectType: 'S'},
			arg:     &pgproto3.Describe{Name: "ping", ObjectType: 'A'},
			wantErr: errors.New("msg => ObjectType: A, want => ObjectType: S"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &expectDescribeMessage{want: tt.field}
			err := e.compare(tt.arg)
			assert.Equal(t, tt.wantErr, err)
		})
	}
}

func Test_expectBindMessage_compare(t *testing.T) {
	tests := []struct {
		name  string
		field *pgproto3.Bind
		arg   pgproto3.FrontendMessage

		wantErr error
	}{
		{
			name:    "success with different in prepared statement",
			field:   &pgproto3.Bind{PreparedStatement: "test"},
			arg:     &pgproto3.Bind{PreparedStatement: "ping"},
			wantErr: nil,
		},
		{
			name:    "success when Parameters nil and empty",
			field:   &pgproto3.Bind{Parameters: [][]byte{}},
			arg:     &pgproto3.Bind{Parameters: nil},
			wantErr: nil,
		},
		{
			name:    "different in type",
			field:   &pgproto3.Bind{},
			arg:     &pgproto3.Parse{},
			wantErr: errors.New("msg => *pgproto3.Parse, want => *pgproto3.Bind"),
		},
		{
			name:    "different in DestinationPortal",
			field:   &pgproto3.Bind{DestinationPortal: "test"},
			arg:     &pgproto3.Bind{DestinationPortal: "ping"},
			wantErr: errors.New("msg => DestinationPortal: ping, want => DestinationPortal: test"),
		},
		{
			name:    "different in ParameterFormatCodes",
			field:   &pgproto3.Bind{ParameterFormatCodes: []int16{1, 2, 3}},
			arg:     &pgproto3.Bind{ParameterFormatCodes: []int16{1, 2, 4}},
			wantErr: errors.New("msg => ParameterFormatCodes: [1 2 4], want => ParameterFormatCodes: [1 2 3]"),
		},
		{
			name:    "different in Parameters",
			field:   &pgproto3.Bind{Parameters: [][]byte{{1, 2, 3}}},
			arg:     &pgproto3.Bind{Parameters: [][]byte{{1, 2, 4}}},
			wantErr: errors.New("msg => Parameters: [[1 2 4]], want => Parameters: [[1 2 3]]"),
		},
		{
			name:    "different in ResultFormatCodes",
			field:   &pgproto3.Bind{ResultFormatCodes: []int16{1, 2, 3}},
			arg:     &pgproto3.Bind{ResultFormatCodes: []int16{1, 2, 4}},
			wantErr: errors.New("msg => ResultFormatCodes: [1 2 4], want => ResultFormatCodes: [1 2 3]"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &expectBindMessage{want: tt.field}
			err := e.compare(tt.arg)
			assert.Equal(t, err, tt.wantErr)
		})
	}
}
