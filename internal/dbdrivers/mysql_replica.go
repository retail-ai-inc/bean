// MIT License

// Copyright (c) The RAI Authors

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package dbdrivers

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"net"
	"sync/atomic"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// replicaRetryInterval is how long reads bypass a replica after it is found unavailable.
const replicaRetryInterval = 10 * time.Second

const (
	mysqlErrTooManyConnections = 1040
	mysqlErrServerShutdown     = 1053
)

// replicaConnPool sends reads to a replica and falls back to the master while the replica is unavailable.
type replicaConnPool struct {
	host      string
	replica   gorm.ConnPool
	master    gorm.ConnPool
	logger    echo.Logger
	now       func() time.Time
	downUntil atomic.Int64
}

func newReplicaConnPool(host string, replica, master gorm.ConnPool, logger echo.Logger) *replicaConnPool {
	return &replicaConnPool{host: host, replica: replica, master: master, logger: logger, now: time.Now}
}

func (p *replicaConnPool) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	if p.available() {
		stmt, err := p.replica.PrepareContext(ctx, query)
		if !p.markUnavailable(err) {
			return stmt, err
		}
	}

	return p.master.PrepareContext(ctx, query)
}

func (p *replicaConnPool) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if p.available() {
		result, err := p.replica.ExecContext(ctx, query, args...)
		if !p.markUnavailable(err) {
			return result, err
		}
	}

	return p.master.ExecContext(ctx, query, args...)
}

func (p *replicaConnPool) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if p.available() {
		rows, err := p.replica.QueryContext(ctx, query, args...)
		if !p.markUnavailable(err) {
			return rows, err
		}
	}

	return p.master.QueryContext(ctx, query, args...)
}

func (p *replicaConnPool) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	if p.available() {
		row := p.replica.QueryRowContext(ctx, query, args...)
		if !p.markUnavailable(row.Err()) {
			return row
		}
	}

	return p.master.QueryRowContext(ctx, query, args...)
}

func (p *replicaConnPool) available() bool {
	return p.now().UnixNano() >= p.downUntil.Load()
}

// markUnavailable reports whether err means the replica cannot serve reads;
// if so, reads bypass the replica for replicaRetryInterval.
func (p *replicaConnPool) markUnavailable(err error) bool {
	if !isReplicaUnavailable(err) {
		return false
	}

	p.downUntil.Store(p.now().Add(replicaRetryInterval).UnixNano())
	if p.logger != nil {
		p.logger.Warnf("mysql read replica %s is unavailable, falling back to master for %s: %v", p.host, replicaRetryInterval, err)
	}

	return true
}

// isReplicaUnavailable matches connection-level failures only. Query errors and
// caller cancellation are returned as-is because the master would fail the same way.
func isReplicaUnavailable(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	if errors.Is(err, driver.ErrBadConn) || errors.Is(err, mysqlDriver.ErrInvalidConn) || errors.Is(err, sql.ErrConnDone) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	var mysqlErr *mysqlDriver.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == mysqlErrTooManyConnections || mysqlErr.Number == mysqlErrServerShutdown
	}

	return false
}
