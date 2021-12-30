{{ .Copyright }}
package repositories

import (
	"{{ .PkgPath }}/framework/internals/global"

	"github.com/labstack/echo/v4"
)

type MyTestRepository interface {
	GetMasterSQLTableList(c echo.Context) (map[string]interface{}, error)
}

func NewMyTestRepository(dbDeps *global.DBDeps) *DbInfra {
	return &DbInfra{dbDeps}
}

func (db *DbInfra) GetMasterSQLTableList(c echo.Context) (map[string]interface{}, error) {

	mysqlDbName := db.Conn.MasterMySQLDBName

	return map[string]interface{}{
		"dbName": mysqlDbName,
	}, nil
}
