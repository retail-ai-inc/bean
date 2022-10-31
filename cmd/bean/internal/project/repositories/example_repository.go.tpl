{{ .Copyright }}
package repositories

import (
	"context"

	"github.com/retail-ai-inc/bean"
	"github.com/retail-ai-inc/bean/trace"
)

type ExampleRepository interface {
	GetMasterSQLTableName(ctx context.Context) (string, error)
}

func NewExampleRepository(dbDeps *bean.DBDeps) *DbInfra {
	return &DbInfra{dbDeps}
}

func (db *DbInfra) GetMasterSQLTableName(ctx context.Context) (string, error) {
	finish := trace.Start(ctx, "db")
	defer finish()
	return db.Conn.MasterMySQLDBName, nil
}