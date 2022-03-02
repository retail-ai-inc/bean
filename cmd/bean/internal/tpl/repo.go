{{ .ProjectObject.Copyright }}
package repositories

import (
	"context"
	// "github.com/retail-ai-inc/bean/helpers"
	// "github.com/getsentry/sentry-go"
	"github.com/retail-ai-inc/bean"
)

type {{.RepoName}}Repository interface{
	{{.RepoName}}ExampleFunc(ctx context.Context) (string, error)
}

func New{{.RepoName}}Repository(dbDeps *bean.DBDeps) *DbInfra {
	return &DbInfra{dbDeps}
}

func (db *DbInfra) {{.RepoName}}ExampleFunc(ctx context.Context) (string, error) {
	// span := sentry.StartSpan(ctx, "repository")
	// span.Description = helpers.CurrFuncName()
	// defer span.Finish()
	return "{{.RepoName}}", nil
}
