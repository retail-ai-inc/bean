package dbdrivers

import (
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func TestTenantString(t *testing.T) {
	cfg := map[string]interface{}{"host": "db", "port": float64(3306), "bad": true, "frac": 1.5, "empty": "", "alt": "alt-db"}

	got, err := tenantString(cfg, "host")
	require.NoError(t, err)
	assert.Equal(t, "db", got)

	got, err = tenantString(cfg, "port")
	require.NoError(t, err)
	assert.Equal(t, "3306", got)

	_, err = tenantString(cfg, "bad")
	assert.ErrorContains(t, err, `"bad" must be a string`)
	_, err = tenantString(cfg, "frac")
	assert.Error(t, err)

	got, err = requiredTenantString(cfg, "empty")
	require.NoError(t, err, "an empty field is allowed, as before")
	assert.Empty(t, got)
	_, err = requiredTenantString(cfg, "missing")
	assert.ErrorContains(t, err, `"missing" is required`)

	got, err = tenantHost(cfg, "alt")
	require.NoError(t, err)
	assert.Equal(t, "alt-db", got)
	got, err = tenantHost(cfg, "")
	require.NoError(t, err)
	assert.Equal(t, "db", got)
}

func tenant(id uint64, connections string) *TenantConnections {
	return &TenantConnections{TenantID: id, Code: fmt.Sprintf("t%d", id), Connections: datatypes.JSON(connections)}
}

// A wrong tenant config returns an error (it panicked), and the connections opened for the previous tenants are
// closed.
func TestGetAllRedisTenantDB_ClosesOpenedConnectionsOnError(t *testing.T) {
	mr := miniredis.RunT(t)
	cfg := RedisConfig{PoolSize: 1, MinIdleConnections: 1, DialTimeout: time.Second}

	tenants := []*TenantConnections{
		// The port may be a JSON number.
		tenant(1, fmt.Sprintf(`{"redis":{"password":"","host":%q,"port":%s,"database":0}}`, mr.Host(), mr.Port())),
		tenant(2, `{"redis":{"password":"","host":true,"port":"6379"}}`),
	}

	var err error
	require.NotPanics(t, func() {
		_, _, err = getAllRedisTenantDB(cfg, tenants, "", "")
	})
	require.ErrorContains(t, err, `invalid redis tenant config (2:t2): "host" must be a string`)

	assert.Eventually(t, func() bool { return mr.CurrentConnectionCount() == 0 }, time.Second, 10*time.Millisecond,
		"the connection of tenant 1 must be closed")
}

func TestGetAllMysqlTenantDB_InvalidConfigIsAnError(t *testing.T) {
	tenants := []*TenantConnections{tenant(1, `{"mysql":{"username":"u","password":"p","host":"h","port":3306,"database":["x"]}}`)}
	var err error
	require.NotPanics(t, func() {
		_, _, _, err = getAllMysqlTenantDB(SQLConfig{}, tenants, "", "", nil)
	})
	assert.ErrorContains(t, err, `invalid mysql tenant config (1:t1): "database" must be a string`)
}

func TestGetAllMongoTenantDB_InvalidConfigIsAnError(t *testing.T) {
	tenants := []*TenantConnections{tenant(1, `{"mongodb":{"username":"u","password":"p","port":"27017","database":"d"}}`)}
	var err error
	require.NotPanics(t, func() {
		_, _, _, err = getAllMongoTenantDB(MongoConfig{}, tenants, "", "", nil)
	})
	assert.ErrorContains(t, err, `invalid mongo tenant config (1:t1): "host" is required`)
}

func TestMongoConnString(t *testing.T) {
	assert.Equal(t, "mongodb://a:27018,b:27018", mongoConnString("a,b", "27018"))
	assert.Equal(t, "mongodb://a:27018,b:27019", mongoConnString("a, b:27019", "27018"))
	assert.Equal(t, "mongodb://[::1]:27017", mongoConnString("::1", "27017"))
}
