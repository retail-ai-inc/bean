{{ .Copyright }}
package services

import (
	"context"

	"{{ .PkgPath }}/repositories"

	"github.com/getsentry/sentry-go"
	"github.com/retail-ai-inc/bean/helpers"
)

type ExampleService interface {
	GetMasterSQLTableList(ctx context.Context) (string, error)
}

type exampleService struct {
	exampleRepository repositories.ExampleRepository
}

func NewExampleService(exampleRepo repositories.ExampleRepository) *exampleService {
	return &exampleService{exampleRepo}
}

func (service *exampleService) GetMasterSQLTableList(ctx context.Context) (string, error) {
	span := sentry.StartSpan(ctx, "service")
	span.Description = helpers.CurrFuncName()
	defer span.Finish()
	return service.exampleRepository.GetMasterSQLTableName(span.Context())
}
