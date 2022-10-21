package pgsnap

import (
	"fmt"
	"reflect"

	"github.com/jackc/pgproto3/v2"
)

type (
	// expectParseMessage is a custom expectation for pgx that ignore Name
	expectParseMessage struct{ want *pgproto3.Parse }

	// expectDescribeMessage is a custom expectation for pgx that ignore Name
	expectDescribeMessage struct{ want *pgproto3.Describe }

	// expectBindMessage is a custom expectation for pgx that ignore PreparedStatement
	expectBindMessage struct{ want *pgproto3.Bind }
)

func (e *expectParseMessage) Step(backend *pgproto3.Backend) error {
	msg, err := backend.Receive()
	if err != nil {
		return err
	}

	m, ok := msg.(*pgproto3.Parse)
	if !ok {
		return fmt.Errorf("msg => %T, e.want => %T", msg, e.want)
	}

	// we ignore and m.Name, because it's inconsisten in pgx
	if m.Query != e.want.Query {
		return fmt.Errorf("msg => query: %s, e.want => query: %s", m.Query, e.want.Query)
	}

	if !reflect.DeepEqual(m.ParameterOIDs, e.want.ParameterOIDs) {
		return fmt.Errorf("msg => ParameterOIDs: %v, e.want => ParameterOIDs: %v", m.ParameterOIDs, e.want.ParameterOIDs)
	}

	return nil
}

func (e *expectDescribeMessage) Step(backend *pgproto3.Backend) error {
	msg, err := backend.Receive()
	if err != nil {
		return err
	}

	m, ok := msg.(*pgproto3.Describe)
	if !ok {
		return fmt.Errorf("msg => %T, e.want => %T", msg, e.want)
	}

	// we ignore and m.Name, because it's inconsisten in pgx
	if m.ObjectType != e.want.ObjectType {
		return fmt.Errorf("msg => query: %s, e.want => query: %s", string(m.ObjectType), string(e.want.ObjectType))
	}

	return nil
}

func (e *expectBindMessage) Step(backend *pgproto3.Backend) error {
	msg, err := backend.Receive()
	if err != nil {
		return err
	}

	m, ok := msg.(*pgproto3.Bind)
	if !ok {
		return fmt.Errorf("msg => %T, e.want => %T", msg, e.want)
	}

	// we ignore and m.Name, because it's inconsisten in pgx
	if m.DestinationPortal != e.want.DestinationPortal {
		return fmt.Errorf(
			"msg => DestinationPortal: %s, e.want => DestinationPortal: %s",
			m.DestinationPortal,
			e.want.DestinationPortal,
		)
	}

	if !reflect.DeepEqual(m.ParameterFormatCodes, e.want.ParameterFormatCodes) {
		return fmt.Errorf(
			"msg => ParameterFormatCodes: %v, e.want => ParameterFormatCodes: %v",
			m.ParameterFormatCodes,
			e.want.ParameterFormatCodes,
		)
	}

	if !reflect.DeepEqual(m.Parameters, e.want.Parameters) {
		return fmt.Errorf(
			"msg => Parameters: %v, e.want => Parameters: %v",
			m.Parameters,
			e.want.Parameters,
		)
	}

	if !reflect.DeepEqual(m.ResultFormatCodes, e.want.ResultFormatCodes) {
		return fmt.Errorf(
			"msg => ResultFormatCodes: %v, e.want => ResultFormatCodes: %v",
			m.ResultFormatCodes,
			e.want.ResultFormatCodes,
		)
	}

	return nil
}