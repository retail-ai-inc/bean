/**#bean*/ /*#bean.replace({{ .Copyright }})**/
package repositories

import (
	"context"

	/**#bean*/
	"demo/framework/bean"
	/*#bean.replace("{{ .PkgPath }}/framework/bean")**/
	/**#bean*/
	"demo/framework/internals/helpers"
	/*#bean.replace("{{ .PkgPath }}/framework/internals/helpers")**/

	"github.com/getsentry/sentry-go"
)

type ExampleRepository interface {
	GetMasterSQLTableName(ctx context.Context) (string, error)
}

func NewExampleRepository(dbDeps *bean.DBDeps) *DbInfra {
	return &DbInfra{dbDeps}
}

func (db *DbInfra) GetMasterSQLTableName(ctx context.Context) (string, error) {
	span := sentry.StartSpan(ctx, "repository")
	span.Description = helpers.CurrFuncName()
	defer span.Finish()
	return db.Conn.MasterMySQLDBName, nil
}
