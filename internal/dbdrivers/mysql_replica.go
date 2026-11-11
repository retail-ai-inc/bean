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

// replicaConnPool sends reads to a replica and falls back to the primary while the replica is unavailable.
type replicaConnPool struct {
	host      string
	replica   gorm.ConnPool
	primary   gorm.ConnPool
	logger    echo.Logger
	now       func() time.Time
	downUntil atomic.Int64
	probing   atomic.Bool
}

var _ gorm.ConnPool = (*replicaConnPool)(nil)

func newReplicaConnPool(host string, replica, primary gorm.ConnPool, logger echo.Logger) *replicaConnPool {
	return &replicaConnPool{host: host, replica: replica, primary: primary, logger: logger, now: time.Now}
}

func (p *replicaConnPool) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	// A prepared statement is bound to the pool that prepared it and cannot
	// safely switch to the primary after a replica failure.
	return p.primary.PrepareContext(ctx, query)
}

func (p *replicaConnPool) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if useReplica, probeUntil := p.tryReplica(); useReplica {
		probe := probeUntil != 0
		if probe {
			defer p.probing.Store(false)
		}
		result, err := p.replica.ExecContext(ctx, query, args...)
		if !p.markUnavailable(err) {
			p.finishProbe(probeUntil)
			return result, err
		}
	}

	return p.primary.ExecContext(ctx, query, args...)
}

func (p *replicaConnPool) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if useReplica, probeUntil := p.tryReplica(); useReplica {
		probe := probeUntil != 0
		if probe {
			defer p.probing.Store(false)
		}
		rows, err := p.replica.QueryContext(ctx, query, args...)
		if !p.markUnavailable(err) {
			p.finishProbe(probeUntil)
			return rows, err
		}
	}

	return p.primary.QueryContext(ctx, query, args...)
}

func (p *replicaConnPool) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	if useReplica, probeUntil := p.tryReplica(); useReplica {
		probe := probeUntil != 0
		if probe {
			defer p.probing.Store(false)
		}
		row := p.replica.QueryRowContext(ctx, query, args...)
		if !p.markUnavailable(row.Err()) {
			p.finishProbe(probeUntil)
			return row
		}
	}

	return p.primary.QueryRowContext(ctx, query, args...)
}

func (p *replicaConnPool) available() bool {
	return p.now().UnixNano() >= p.downUntil.Load()
}

// tryReplica allows all reads while the replica is healthy. After a failure,
// only one request probes it when the retry interval expires; concurrent reads
// continue using the primary until that probe completes.
func (p *replicaConnPool) tryReplica() (useReplica bool, probeUntil int64) {
	downUntil := p.downUntil.Load()
	if downUntil == 0 {
		return true, 0
	}
	if p.now().UnixNano() < downUntil {
		return false, 0
	}
	if p.probing.CompareAndSwap(false, true) {
		return true, downUntil
	}
	return false, 0
}

func (p *replicaConnPool) finishProbe(probeUntil int64) {
	if probeUntil != 0 {
		// Do not erase a newer failure recorded by another in-flight request.
		p.downUntil.CompareAndSwap(probeUntil, 0)
	}
}

// markUnavailable reports whether err means the replica cannot serve reads;
// if so, reads bypass the replica for replicaRetryInterval.
func (p *replicaConnPool) markUnavailable(err error) bool {
	if !isReplicaUnavailable(err) {
		return false
	}

	p.setUnavailable(err)
	return true
}

func (p *replicaConnPool) setUnavailable(err error) {
	p.downUntil.Store(p.now().Add(replicaRetryInterval).UnixNano())
	if p.logger != nil {
		p.logger.Warnf("mysql read replica %s is unavailable, falling back to primary for %s: %v", p.host, replicaRetryInterval, err)
	}
}

// isReplicaUnavailable matches connection-level failures only. Query errors and
// caller cancellation are returned as-is because the primary would fail the same way.
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
