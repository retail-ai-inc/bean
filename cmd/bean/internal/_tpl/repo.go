package repositories

import (
	"context"

	// "github.com/retail-ai-inc/bean/v2/trace"
	"github.com/retail-ai-inc/bean/v2"
)

type {{.RepoNameUpper}}Repository interface {
	{{.RepoNameUpper}}ExampleFunc(ctx context.Context) (string, error)
}

func New{{.RepoNameUpper}}Repository(dbDeps *bean.DBDeps) *DbInfra {
	return &DbInfra{dbDeps}
}

func (db *DbInfra) {{.RepoNameUpper}}ExampleFunc(ctx context.Context) (string, error) {
	// IMPORTANT: If you wanna trace the performance of your handler function then uncomment following 2 lines
	// _, finish := trace.StartSpan(ctx, "db")
	// defer finish()
	return "{{.RepoNameUpper}}", nil
}
