/*

Package pgtest starts and stops a postgres server, quickly
and conveniently, for Go unit tests. To use it:

	func TestSomething(t *testing.T) {
		defer pgtest.Start(t).Stop()
		db, err := sql.Open("postgres", pgtest.URL)
		// etc.
	}

This package is not very configurable, though it may become
so in the future.

*/
package pgtest

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

const (
	// Postgres data directory to copy for new postgres processes.
	// Dir will be created with initdb if it doesn't already exist.
	Dir = "pgtestdata"

	// Connection URL for sql.Open, to connect to a database
	// started by this package.
	URL = "host=/tmp dbname=postgres sslmode=disable"

	// to poll until postgres fully starts
	sock = "/tmp/.s.PGSQL.5432"

	conf = `
fsync = off
listen_addresses = ''
unix_socket_directory = '/tmp'
`
)

var once sync.Once

type PG struct {
	t   *testing.T
	dir string
	cmd *exec.Cmd
}

// Start runs postgres by copying data directory Dir to a temporary
// location, then running postgres in that temporary directory.
// If an error occurs, the test will fail.
func Start(t *testing.T) *PG {
	once.Do(func() { maybeInitdb(t) })
	var err error
	pg := new(PG)
	pg.t = t
	pg.dir, err = ioutil.TempDir("", "pgtest")
	if err != nil {
		t.Fatal(err)
	}
	err = exec.Command("cp", "-a", Dir+"/.", pg.dir).Run()
	if err != nil {
		t.Fatal("copy:", err)
	}
	pg.cmd = exec.Command("postgres", "-D", pg.dir)
	err = pg.cmd.Start()
	if err != nil {
		t.Fatal("starting postgres:", err)
	}
	for n := 0; n < 20; n++ {
		if _, err := os.Stat(sock); err == nil {
			return pg
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("timeout waiting for postgres to start")
	panic("unreached")
}

// Stop stops the running postgres process and removes its
// temporary data directory.
// If an error occurs, the test will fail.
func (pg *PG) Stop() {
	err := pg.cmd.Process.Signal(os.Interrupt)
	if err != nil {
		pg.t.Fatal("postgres:", err)
	}
	err = os.RemoveAll(pg.dir)
	if err != nil {
		pg.t.Fatal(err)
	}
}

func maybeInitdb(t *testing.T) {
	err := os.Mkdir(Dir, 0777)
	if os.IsExist(err) {
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	err = exec.Command("initdb", "-D", Dir).Run()
	if err != nil {
		t.Fatal("initdb", err)
	}
	path := filepath.Join(Dir, "postgresql.conf")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.Write([]byte(conf))
	if err != nil {
		t.Fatal(err)
	}
	err = f.Close()
	if err != nil {
		t.Fatal(err)
	}
}
