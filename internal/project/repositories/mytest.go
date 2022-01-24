/**#bean*/ /*#bean.replace({{ .Copyright }})**/
package repositories

import (
	/**#bean*/
	"demo/framework/bean"
	/*#bean.replace("{{ .PkgPath }}/framework/bean")**/

	"github.com/labstack/echo/v4"
)

type MyTestRepository interface {
	GetMasterSQLTableName(c echo.Context) (string, error)
}

func NewMyTestRepository(dbDeps *bean.DBDeps) *DbInfra {
	return &DbInfra{dbDeps}
}

func (db *DbInfra) GetMasterSQLTableName(c echo.Context) (string, error) {
	return db.Conn.MasterMySQLDBName, nil
}
