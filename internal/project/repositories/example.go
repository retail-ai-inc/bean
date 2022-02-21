{{ .Copyright }})
package repositories

import (
	"context"

	"github.com/getsentry/sentry-go"
	"github.com/retail-ai-inc/bean/framework/bean"
	"github.com/retail-ai-inc/bean/framework/helpers"
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
