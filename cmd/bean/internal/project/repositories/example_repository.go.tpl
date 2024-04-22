{{ .Copyright }}
package repositories

import (
	"context"

	"github.com/retail-ai-inc/bean/v2"
	"github.com/retail-ai-inc/bean/v2/trace"
)

type ExampleRepository interface {
	GetMasterSQLTableName(ctx context.Context) (string, error)
}

func NewExampleRepository(dbDeps *bean.DBDeps) *DbInfra {
	return &DbInfra{dbDeps}
}

func (db *DbInfra) GetMasterSQLTableName(ctx context.Context) (string, error) {
	_, finish := trace.StartSpan(ctx, "db")
	defer finish()
	return db.Conn.MasterMySQLDBName, nil
}