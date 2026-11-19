package dbdrivers

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"net"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestIsReplicaUnavailable(t *testing.T) {
	dialErr := &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "context canceled", err: context.Canceled, want: false},
		{name: "context deadline exceeded", err: fmt.Errorf("query: %w", context.DeadlineExceeded), want: false},
		{name: "dial timed out by caller deadline", err: &net.OpError{Op: "dial", Err: context.DeadlineExceeded}, want: false},
		{name: "bad connection", err: fmt.Errorf("query: %w", driver.ErrBadConn), want: true},
		{name: "invalid connection", err: mysqlDriver.ErrInvalidConn, want: true},
		{name: "connection done", err: sql.ErrConnDone, want: true},
		{name: "dial refused", err: dialErr, want: true},
		{name: "dns failure", err: &net.DNSError{Err: "no such host", Name: "replica"}, want: true},
		{name: "too many connections", err: &mysqlDriver.MySQLError{Number: mysqlErrTooManyConnections}, want: true},
		{name: "server shutdown", err: &mysqlDriver.MySQLError{Number: mysqlErrServerShutdown}, want: true},
		{name: "unknown table", err: &mysqlDriver.MySQLError{Number: 1146}, want: false},
		{name: "plain error", err: errors.New("boom"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isReplicaUnavailable(tt.err))
		})
	}
}

func newMockSQLDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	return sqlDB, mock
}

func TestReplicaConnPoolFallback(t *testing.T) {
	const query = "SELECT 1"
	quoted := regexp.QuoteMeta(query)
	ctx := context.Background()
	unavailableErr := &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}
	queryErr := &mysqlDriver.MySQLError{Number: 1146, Message: "table doesn't exist"}

	methods := []struct {
		name   string
		expect func(mock sqlmock.Sqlmock, err error)
		call   func(p *replicaConnPool) error
	}{
		{
			name: "query",
			expect: func(mock sqlmock.Sqlmock, err error) {
				if err != nil {
					mock.ExpectQuery(quoted).WillReturnError(err)
					return
				}
				mock.ExpectQuery(quoted).WillReturnRows(sqlmock.NewRows([]string{"v"}).AddRow(1))
			},
			call: func(p *replicaConnPool) error {
				rows, err := p.QueryContext(ctx, query)
				if rows != nil {
					_ = rows.Close()
				}
				return err
			},
		},
		{
			name: "query row",
			expect: func(mock sqlmock.Sqlmock, err error) {
				if err != nil {
					mock.ExpectQuery(quoted).WillReturnError(err)
					return
				}
				mock.ExpectQuery(quoted).WillReturnRows(sqlmock.NewRows([]string{"v"}).AddRow(1))
			},
			call: func(p *replicaConnPool) error {
				var v int
				return p.QueryRowContext(ctx, query).Scan(&v)
			},
		},
		{
			name: "exec",
			expect: func(mock sqlmock.Sqlmock, err error) {
				if err != nil {
					mock.ExpectExec(quoted).WillReturnError(err)
					return
				}
				mock.ExpectExec(quoted).WillReturnResult(sqlmock.NewResult(0, 0))
			},
			call: func(p *replicaConnPool) error {
				_, err := p.ExecContext(ctx, query)
				return err
			},
		},
	}

	for _, m := range methods {
		t.Run(m.name+"/healthy replica serves the read", func(t *testing.T) {
			replicaDB, replicaMock := newMockSQLDB(t)
			masterDB, masterMock := newMockSQLDB(t)
			pool := newReplicaConnPool("replica", replicaDB, masterDB, nil)

			m.expect(replicaMock, nil)

			require.NoError(t, m.call(pool))
			assert.NoError(t, replicaMock.ExpectationsWereMet())
			assert.NoError(t, masterMock.ExpectationsWereMet())
		})

		t.Run(m.name+"/unavailable replica falls back until retry interval passes", func(t *testing.T) {
			replicaDB, replicaMock := newMockSQLDB(t)
			masterDB, masterMock := newMockSQLDB(t)
			now := time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)
			pool := newReplicaConnPool("replica", replicaDB, masterDB, nil)
			pool.now = func() time.Time { return now }

			m.expect(replicaMock, unavailableErr)
			m.expect(masterMock, nil)
			require.NoError(t, m.call(pool), "failed replica read must be served by master")

			m.expect(masterMock, nil)
			now = now.Add(replicaRetryInterval - time.Nanosecond)
			require.NoError(t, m.call(pool), "replica must be bypassed during the retry interval")
			require.NoError(t, masterMock.ExpectationsWereMet())

			m.expect(replicaMock, nil)
			now = now.Add(time.Nanosecond)
			require.NoError(t, m.call(pool), "replica must be retried after the retry interval")

			assert.NoError(t, replicaMock.ExpectationsWereMet())
			assert.NoError(t, masterMock.ExpectationsWereMet())
		})

		t.Run(m.name+"/query error is returned without fallback", func(t *testing.T) {
			replicaDB, replicaMock := newMockSQLDB(t)
			masterDB, masterMock := newMockSQLDB(t)
			pool := newReplicaConnPool("replica", replicaDB, masterDB, nil)

			m.expect(replicaMock, queryErr)

			err := m.call(pool)
			assert.ErrorIs(t, err, queryErr)
			assert.True(t, pool.available(), "query errors must not mark the replica unavailable")
			assert.NoError(t, replicaMock.ExpectationsWereMet())
			assert.NoError(t, masterMock.ExpectationsWereMet())
		})
	}
}

func TestReplicaConnPoolPrepareUsesMaster(t *testing.T) {
	const query = "SELECT 1"
	replicaDB, replicaMock := newMockSQLDB(t)
	masterDB, masterMock := newMockSQLDB(t)
	pool := newReplicaConnPool("replica", replicaDB, masterDB, nil)

	masterMock.ExpectPrepare(regexp.QuoteMeta(query))
	stmt, err := pool.PrepareContext(context.Background(), query)
	require.NoError(t, err)
	require.NoError(t, stmt.Close())
	assert.NoError(t, replicaMock.ExpectationsWereMet())
	assert.NoError(t, masterMock.ExpectationsWereMet())
}

func TestReplicaConnPoolAllowsOneRecoveryProbe(t *testing.T) {
	replicaDB, _ := newMockSQLDB(t)
	masterDB, _ := newMockSQLDB(t)
	now := time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)
	pool := newReplicaConnPool("replica", replicaDB, masterDB, nil)
	pool.now = func() time.Time { return now }
	pool.downUntil.Store(now.Add(-time.Second).UnixNano())

	useReplica, probeUntil := pool.tryReplica()
	assert.True(t, useReplica)
	assert.NotZero(t, probeUntil)
	useReplica, _ = pool.tryReplica()
	assert.False(t, useReplica, "a concurrent request must keep using the primary")

	pool.finishProbe(probeUntil)
	pool.probing.Store(false)
	useReplica, probeUntil = pool.tryReplica()
	assert.True(t, useReplica, "a successful probe restores normal replica reads")
	assert.Zero(t, probeUntil)
	useReplica, probeUntil = pool.tryReplica()
	assert.True(t, useReplica, "healthy replicas do not serialize reads")
	assert.Zero(t, probeUntil)
}

func TestUseMysqlReplicasFallsBackToMaster(t *testing.T) {
	masterDB, masterMock := newMockSQLDB(t)
	replicaDB, replicaMock := newMockSQLDB(t)

	db, err := gorm.Open(mysql.New(mysql.Config{Conn: masterDB, SkipInitializeWithVersion: true}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)

	pool := newReplicaConnPool("replica", replicaDB, masterDB, nil)
	require.NoError(t, useMysqlReplicas(db, []gorm.Dialector{
		mysql.New(mysql.Config{Conn: pool, SkipInitializeWithVersion: true}),
	}))

	selectUsers := regexp.QuoteMeta("SELECT * FROM `users`")
	replicaMock.ExpectQuery(selectUsers).WillReturnError(&net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")})
	masterMock.ExpectQuery(selectUsers).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	var rows []map[string]any
	require.NoError(t, db.Table("users").Find(&rows).Error)
	assert.Len(t, rows, 1)
	assert.NoError(t, replicaMock.ExpectationsWereMet())
	assert.NoError(t, masterMock.ExpectationsWereMet())
}
