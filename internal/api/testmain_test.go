package api

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
)

// See the matching comment in internal/account/testmain_test.go — this
// lock ID must match so the two packages actually serialize against each
// other instead of each just locking against itself.
const testDBLockID = 846213579

func TestMain(m *testing.M) {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, "postgres://wallet:wallet@localhost:5433/wallet")
	if err != nil {
		fmt.Println("failed to connect for test lock:", err)
		os.Exit(1)
	}
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", testDBLockID); err != nil {
		fmt.Println("failed to acquire test lock:", err)
		os.Exit(1)
	}

	code := m.Run()

	conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", testDBLockID)
	conn.Close(ctx)
	os.Exit(code)
}
