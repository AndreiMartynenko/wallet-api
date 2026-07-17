package account

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
)

// testDBLockID is a Postgres advisory lock shared by every package's test
// suite that talks to the dev Postgres instance (see the matching
// TestMain in internal/api). go test ./... runs different packages'
// tests in parallel by default; without this lock, one package's
// `DELETE FROM accounts` cleanup could wipe rows another package's
// still-running test just created, causing intermittent
// "account not found" failures that have nothing to do with the code
// under test.
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
