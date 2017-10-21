// +build go1.8

package proxy

import (
	"context"
	"database/sql/driver"
	"fmt"
	"io"
	"sync"
)

type loggingHook struct {
	io.Writer
	mu sync.Mutex
}

func newLoggingHook(w io.Writer) *loggingHook {
	return &loggingHook{
		Writer: w,
	}
}

func (h *loggingHook) prePing(c context.Context, conn *Conn) (interface{}, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	fmt.Fprintln(h, "[PrePing]")
	return nil, nil
}

func (h *loggingHook) ping(c context.Context, ctx interface{}, conn *Conn) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	fmt.Fprintln(h, "[Ping]")
	return nil
}

func (h *loggingHook) postPing(c context.Context, ctx interface{}, conn *Conn, err error) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	fmt.Fprintln(h, "[PostPing]")
	return nil
}

func (h *loggingHook) preOpen(c context.Context, name string) (interface{}, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	fmt.Fprintln(h, "[PreOpen]")
	return nil, nil
}

func (h *loggingHook) open(c context.Context, ctx interface{}, conn driver.Conn) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	fmt.Fprintln(h, "[Open]")
	return nil
}

func (h *loggingHook) postOpen(c context.Context, ctx interface{}, conn driver.Conn, err error) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	fmt.Fprintln(h, "[PostOpen]")
	return nil
}

func (h *loggingHook) preExec(c context.Context, stmt *Stmt, args []driver.NamedValue) (interface{}, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	fmt.Fprintln(h, "[PreExec]")
	return nil, nil
}

func (h *loggingHook) exec(c context.Context, ctx interface{}, stmt *Stmt, args []driver.NamedValue, result driver.Result) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	fmt.Fprintln(h, "[Exec]")
	return nil
}

func (h *loggingHook) postExec(c context.Context, ctx interface{}, stmt *Stmt, args []driver.NamedValue, result driver.Result, err error) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	fmt.Fprintln(h, "[PostExec]")
	return nil
}

func (h *loggingHook) preQuery(c context.Context, stmt *Stmt, args []driver.NamedValue) (interface{}, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	fmt.Fprintln(h, "[PreQuery]")
	return nil, nil
}

func (h *loggingHook) query(c context.Context, ctx interface{}, stmt *Stmt, args []driver.NamedValue, rows driver.Rows) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	fmt.Fprintln(h, "[Query]")
	return nil
}

func (h *loggingHook) postQuery(c context.Context, ctx interface{}, stmt *Stmt, args []driver.NamedValue, rows driver.Rows, err error) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	fmt.Fprintln(h, "[PostQuery]")
	return nil
}

func (h *loggingHook) preBegin(c context.Context, conn *Conn) (interface{}, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	fmt.Fprintln(h, "[PreBegin]")
	return nil, nil
}

func (h *loggingHook) begin(c context.Context, ctx interface{}, conn *Conn) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	fmt.Fprintln(h, "[Begin]")
	return nil
}

func (h *loggingHook) postBegin(c context.Context, ctx interface{}, conn *Conn, err error) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	fmt.Fprintln(h, "[PostBegin]")
	return nil
}

func (h *loggingHook) preCommit(c context.Context, tx *Tx) (interface{}, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	fmt.Fprintln(h, "[PreCommit]")
	return nil, nil
}

func (h *loggingHook) commit(c context.Context, ctx interface{}, tx *Tx) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	fmt.Fprintln(h, "[Commit]")
	return nil
}

func (h *loggingHook) postCommit(c context.Context, ctx interface{}, tx *Tx, err error) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	fmt.Fprintln(h, "[PostCommit]")
	return nil
}

func (h *loggingHook) preRollback(c context.Context, tx *Tx) (interface{}, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	fmt.Fprintln(h, "[PreRollback]")
	return nil, nil
}

func (h *loggingHook) rollback(c context.Context, ctx interface{}, tx *Tx) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	fmt.Fprintln(h, "[Rollback]")
	return nil
}

func (h *loggingHook) postRollback(c context.Context, ctx interface{}, tx *Tx, err error) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	fmt.Fprintln(h, "[PostRollback]")
	return nil
}
