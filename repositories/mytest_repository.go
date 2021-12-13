/*
 * Copyright The RAI Inc.
 * The RAI Authors
 */

package repositories

import (
	"bean/internals/global"

	"github.com/labstack/echo/v4"
)

func NewMyTestRepository(dbDeps *global.DBDeps) *DbInfra {
	return &DbInfra{dbDeps}
}

func (db *DbInfra) GetMasterSQLTableList(c echo.Context) (map[string]interface{}, error) {

	mysqlDbName := db.Conn.MasterMySQLDBName

	return map[string]interface{}{
		"dbName": mysqlDbName,
	}, nil
}
