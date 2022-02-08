/**#bean*/ /*#bean.replace({{ .Copyright }})**/
package services

import (
	"context"

	/**#bean*/
	"demo/framework/internals/helpers"
	/*#bean.replace("{{ .PkgPath }}/framework/internals/helpers")**/
	/**#bean*/
	"demo/repositories"
	/*#bean.replace("{{ .PkgPath }}/repositories")**/

	"github.com/getsentry/sentry-go"
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
