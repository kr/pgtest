/*

Package pgtest starts and stops a postgres server, quickly
and conveniently, for Go unit tests. To use it:

	func TestSomething(t *testing.T) {
		pg := pgtest.Start(t)
		defer pg.Stop()
		db, err := sql.Open("postgres", pg.URL)
		// etc.
	}

This package is not very configurable, though it may become
so in the future.

*/
package pgtest

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"text/template"
	"time"
)

var conf = template.Must(template.New("t").Parse(`
fsync = off
listen_addresses = ''

{{if .Plural}}
unix_socket_directories = '{{.ConfDir}}'
{{else}}
unix_socket_directory = '{{.ConfDir}}'
{{end}}

`))

var pgtestdata = filepath.Join(os.TempDir(), "pgtestdata1")

var (
	postgres string
	initdbOk = false
	once     sync.Once
)

type PG struct {
	URL string // Connection URL for sql.Open.
	t   *testing.T
	dir string
	cmd *exec.Cmd
}

// Start runs postgres in a temporary directory,
// with a default file set produced by initdb.
// If an error occurs, the test will fail.
func Start(t *testing.T) *PG {
	once.Do(func() { maybeInitdb(t) })
	if !initdbOk {
		t.Fatal("prior initdb attempt failed")
	}
	var err error
	pg := new(PG)
	pg.t = t
	pg.dir, err = ioutil.TempDir("", "pgtest")
	if err != nil {
		t.Fatal(err)
	}
	err = exec.Command("cp", "-a", pgtestdata+"/.", pg.dir).Run()
	if err != nil {
		t.Fatal("copy:", err)
	}
	path := filepath.Join(pg.dir, "postgresql.conf")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		t.Fatal(err)
	}
	plural := !contains("unix_socket_directory", path)
	err = conf.Execute(f, struct {
		ConfDir string
		Plural  bool
	}{pg.dir, plural})
	if err != nil {
		t.Fatal(err)
	}
	err = f.Close()
	if err != nil {
		t.Fatal(err)
	}
	pg.URL = "host=" + pg.dir + " dbname=postgres sslmode=disable"
	pg.cmd = exec.Command(postgres, "-D", pg.dir)
	err = pg.cmd.Start()
	if err != nil {
		t.Fatal("starting postgres:", err)
	}
	sock := filepath.Join(pg.dir, ".s.PGSQL.5432")
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
	out, err := exec.Command("pg_config", "--bindir").Output()
	if err != nil {
		t.Fatal("pg_config", err)
	}
	bindir := string(bytes.TrimSpace(out))
	postgres = filepath.Join(bindir, "postgres")
	initdb := filepath.Join(bindir, "initdb")
	err = os.Mkdir(pgtestdata, 0777)
	if os.IsExist(err) {
		initdbOk = true
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	err = exec.Command(initdb, "-D", pgtestdata).Run()
	if err != nil {
		os.RemoveAll(pgtestdata)
		t.Fatal("initdb", err)
	}
	initdbOk = true
}

func contains(substr, name string) bool {
	b, err := ioutil.ReadFile(name)
	return err == nil && bytes.Contains(b, []byte(substr))
}
