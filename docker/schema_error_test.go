package docker

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func TestFailedCausedByWrongSQL(t *testing.T) {
	snap := NewPgSnapDocker(
		t,
		WithMigrationPath("./wrong_schema"),
		//WithDebug(),
	)
	defer snap.Finish()

	db, err := pgx.Connect(context.TODO(), snap.Addr())
	require.NoError(t, err)

	err = db.Ping(context.TODO())
	require.NoError(t, err)
}
