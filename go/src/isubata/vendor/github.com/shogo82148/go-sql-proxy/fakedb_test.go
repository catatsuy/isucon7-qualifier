// +build go1.8

package proxy

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

type fakeConnOption struct {
	// name is the name of database
	Name string

	// ConnType enhances the driver implementation
	ConnType string

	// call of Exec will fail if failExec is true
	FailExec bool

	// call of Query will fail if failQuery is true
	FailQuery bool
}

type fakeDriver struct {
	mu  sync.Mutex
	dbs map[string]*fakeDB
}

type fakeDB struct {
	mu  sync.Mutex
	log *bytes.Buffer
}

// fakeConn is minimum implementation of driver.Conn
type fakeConn struct {
	db  *fakeDB
	opt *fakeConnOption
}

// fakeConnExt implements Execer and Queryer
type fakeConnExt fakeConn

// fakeConnCtx is fakeConn with context
type fakeConnCtx fakeConn

type fakeTx struct {
	db  *fakeDB
	opt *fakeConnOption
}

// fakeStmt is minimum implementation of driver.Stmt
type fakeStmt struct {
	db  *fakeDB
	opt *fakeConnOption
}

// fakeStmtExt implements ColumnConverter
type fakeStmtExt fakeStmt

// fakeStmtCtx is fakeStmt with context
type fakeStmtCtx fakeStmt

var fdriver = &fakeDriver{}
var _ driver.Driver = &fakeDriver{}
var _ driver.Conn = &fakeConn{}
var _ driver.Conn = &fakeConnExt{}
var _ driver.Execer = &fakeConnExt{}
var _ driver.Queryer = &fakeConnExt{}
var _ driver.Conn = &fakeConnCtx{}
var _ driver.Execer = &fakeConnCtx{}
var _ driver.ExecerContext = &fakeConnCtx{}
var _ driver.Queryer = &fakeConnCtx{}
var _ driver.QueryerContext = &fakeConnCtx{}
var _ driver.ConnBeginTx = &fakeConnCtx{}
var _ driver.ConnPrepareContext = &fakeConnCtx{}
var _ driver.Pinger = &fakeConnCtx{}
var _ driver.Tx = &fakeTx{}
var _ driver.Stmt = &fakeStmt{}
var _ driver.Stmt = &fakeStmtExt{}
var _ driver.ColumnConverter = &fakeStmtExt{}
var _ driver.Stmt = &fakeStmtCtx{}
var _ driver.ColumnConverter = &fakeStmtCtx{}
var _ driver.StmtExecContext = &fakeStmtCtx{}
var _ driver.StmtQueryContext = &fakeStmtCtx{}

func init() {
	sql.Register("fakedb", fdriver)
}

func (d *fakeDriver) Open(name string) (driver.Conn, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	var opt fakeConnOption
	err := json.Unmarshal([]byte(name), &opt)
	if err != nil {
		return nil, err
	}

	db, ok := d.dbs[opt.Name]
	if !ok {
		db = &fakeDB{
			log: &bytes.Buffer{},
		}
		if d.dbs == nil {
			d.dbs = make(map[string]*fakeDB)
		}
		d.dbs[name] = db
	}

	var conn driver.Conn
	switch opt.ConnType {
	case "", "fakeConn":
		conn = &fakeConn{
			db:  db,
			opt: &opt,
		}
	case "fakeConnExt":
		conn = &fakeConnExt{
			db:  db,
			opt: &opt,
		}
	case "fakeConnCtx":
		conn = &fakeConnCtx{
			db:  db,
			opt: &opt,
		}
	default:
		return nil, errors.New("known ConnType")
	}

	return conn, nil
}

func (d *fakeDriver) DB(name string) *fakeDB {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.dbs[name]
}

// Log write the params to the log.
func (db *fakeDB) Log(params ...interface{}) {
	db.mu.Lock()
	defer db.mu.Unlock()
	fmt.Fprintln(db.log, params...)
}

// LogToString converts the log into string.
func (db *fakeDB) LogToString() string {
	db.mu.Lock()
	defer db.mu.Unlock()
	return db.log.String()
}

func (c *fakeConn) Prepare(query string) (driver.Stmt, error) {
	c.db.Log("[Conn.Prepare]", query)
	return &fakeStmt{
		db:  c.db,
		opt: c.opt,
	}, nil
}

func (c *fakeConn) Close() error {
	return nil
}

func (c *fakeConn) Begin() (driver.Tx, error) {
	c.db.Log("[Conn.Begin]")
	return &fakeTx{
		db:  c.db,
		opt: c.opt,
	}, nil
}

func (c *fakeConnExt) Prepare(query string) (driver.Stmt, error) {
	c.db.Log("[Conn.Prepare]", query)
	return &fakeStmtExt{
		db:  c.db,
		opt: c.opt,
	}, nil
}

func (c *fakeConnExt) Close() error {
	return nil
}

func (c *fakeConnExt) Begin() (driver.Tx, error) {
	c.db.Log("[Conn.Begin]")
	return &fakeTx{
		db:  c.db,
		opt: c.opt,
	}, nil
}

func (c *fakeConnExt) Exec(query string, args []driver.Value) (driver.Result, error) {
	c.db.Log("[Conn.Exec]", query, convertValuesToString(args))
	if c.opt.FailExec {
		c.db.Log("[Conn.Exec]", "ERROR!")
		return nil, errors.New("Exec failed")
	}
	return nil, nil
}

func (c *fakeConnExt) Query(query string, args []driver.Value) (driver.Rows, error) {
	c.db.Log("[Conn.Query]", query, convertValuesToString(args))
	if c.opt.FailQuery {
		c.db.Log("[Conn.Query]", "ERROR!")
		return nil, errors.New("Query failed")
	}
	return nil, nil
}

func (c *fakeConnCtx) Ping(ctx context.Context) error {
	c.db.Log("[Conn.Ping]")
	return nil
}

func (c *fakeConnCtx) Prepare(query string) (driver.Stmt, error) {
	panic("not supported")
}

func (c *fakeConnCtx) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	c.db.Log("[Conn.PrepareContext]", query)
	return &fakeStmtCtx{
		db:  c.db,
		opt: c.opt,
	}, nil
}

func (c *fakeConnCtx) Close() error {
	return nil
}

func (c *fakeConnCtx) Begin() (driver.Tx, error) {
	panic("not supported")
}

func (c *fakeConnCtx) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	c.db.Log("[Conn.BeginTx]")
	return &fakeTx{
		db:  c.db,
		opt: c.opt,
	}, nil
}

func (c *fakeConnCtx) Exec(query string, args []driver.Value) (driver.Result, error) {
	panic("not supported")
}

func (c *fakeConnCtx) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	c.db.Log("[Conn.ExecContext]", query, convertNamedValuesToString(args))
	if c.opt.FailExec {
		c.db.Log("[Conn.ExecContext]", "ERROR!")
		return nil, errors.New("Exec failed")
	}
	return nil, nil
}

func (c *fakeConnCtx) Query(query string, args []driver.Value) (driver.Rows, error) {
	panic("not supported")
}

func (c *fakeConnCtx) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.db.Log("[Conn.QueryContext]", query, convertNamedValuesToString(args))
	if c.opt.FailQuery {
		c.db.Log("[Conn.QueryContext]", "ERROR!")
		return nil, errors.New("Query failed")
	}
	return nil, nil
}

func (tx *fakeTx) Commit() error {
	tx.db.Log("[Tx.Commit]")
	return nil
}

func (tx *fakeTx) Rollback() error {
	tx.db.Log("[Tx.Rollback]")
	return nil
}

func (stmt *fakeStmt) Close() error {
	stmt.db.Log("[Stmt.Close]")
	return nil
}

func (stmt *fakeStmt) NumInput() int {
	return -1 // fakeDriver doesn't know its number of placeholders
}

func (stmt *fakeStmt) Exec(args []driver.Value) (driver.Result, error) {
	stmt.db.Log("[Stmt.Exec]", convertValuesToString(args))
	if stmt.opt.FailExec {
		stmt.db.Log("[Stmt.Exec]", "ERROR!")
		return nil, errors.New("Exec failed")
	}
	return nil, nil
}

func (stmt *fakeStmt) Query(args []driver.Value) (driver.Rows, error) {
	stmt.db.Log("[Stmt.Query]", convertValuesToString(args))
	if stmt.opt.FailQuery {
		stmt.db.Log("[Stmt.Query]", "ERROR!")
		return nil, errors.New("Query failed")
	}
	return nil, nil
}

func (stmt *fakeStmtExt) Close() error {
	stmt.db.Log("[Stmt.Close]")
	return nil
}

func (stmt *fakeStmtExt) NumInput() int {
	return -1 // fakeDriver doesn't know its number of placeholders
}

func (stmt *fakeStmtExt) Exec(args []driver.Value) (driver.Result, error) {
	return (*fakeStmt)(stmt).Exec(args)
}

func (stmt *fakeStmtExt) Query(args []driver.Value) (driver.Rows, error) {
	return (*fakeStmt)(stmt).Query(args)
}

func (stmt *fakeStmtExt) ColumnConverter(idx int) driver.ValueConverter {
	stmt.db.Log("[Stmt.ColumnConverter]", idx)
	return driver.DefaultParameterConverter
}

func (stmt *fakeStmtCtx) Close() error {
	stmt.db.Log("[Stmt.Close]")
	return nil
}

func (stmt *fakeStmtCtx) NumInput() int {
	return -1 // fakeDriver doesn't know its number of placeholders
}

func (stmt *fakeStmtCtx) Exec(args []driver.Value) (driver.Result, error) {
	panic("not supported")
}

func (stmt *fakeStmtCtx) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	stmt.db.Log("[Conn.ExecContext]", convertNamedValuesToString(args))
	if stmt.opt.FailExec {
		stmt.db.Log("[Conn.ExecContext]", "ERROR!")
		return nil, errors.New("Exec failed")
	}
	return nil, nil
}

func (stmt *fakeStmtCtx) Query(args []driver.Value) (driver.Rows, error) {
	panic("not supported")
}

func (stmt *fakeStmtCtx) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	stmt.db.Log("[Conn.QueryContext]", convertNamedValuesToString(args))
	if stmt.opt.FailQuery {
		stmt.db.Log("[Conn.QueryContext]", "ERROR!")
		return nil, errors.New("Query failed")
	}
	return nil, nil
}

func (stmt *fakeStmtCtx) ColumnConverter(idx int) driver.ValueConverter {
	stmt.db.Log("[Stmt.ColumnConverter]", idx)
	return driver.DefaultParameterConverter
}

func convertValuesToString(args []driver.Value) string {
	buf := new(bytes.Buffer)
	for _, arg := range args {
		fmt.Fprintf(buf, " %#v", arg)
	}
	return buf.String()
}

func convertNamedValuesToString(args []driver.NamedValue) string {
	buf := new(bytes.Buffer)
	for _, arg := range args {
		fmt.Fprintf(buf, " %#v", arg)
	}
	return buf.String()
}
