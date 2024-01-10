{{ .Copyright }}
package services

import (
	"context"

	"{{ .PkgPath }}/repositories"

	"github.com/retail-ai-inc/bean/v2/trace"
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
	finish := trace.Start(ctx, "http.service")
	defer finish()
	return service.exampleRepository.GetMasterSQLTableName(ctx)
}
