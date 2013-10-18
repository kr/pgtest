package pgtest

import (
	"database/sql"
	_ "github.com/lib/pq"
	"testing"
)

func TestRun(t *testing.T) {
	defer Start(t).Stop()

	db, err := sql.Open("postgres", URL)
	if err != nil {
		t.Fatal("open", err)
	}
	var n int
	err = db.QueryRow("SELECT 1").Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatal("SELECT 1 = %d", n)
	}
}
