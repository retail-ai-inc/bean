/**#bean*/ /*#bean.replace({{ .Copyright }})**/
package services

import (
	/**#bean*/
	"demo/repositories"
	/*#bean.replace("{{ .PkgPath }}/repositories")**/

	"github.com/labstack/echo/v4"
)

type MyTestService interface {
	GetMasterSQLTableList(c echo.Context) (string, error)
}

type myTestService struct {
	myTestRepository repositories.MyTestRepository
}

func NewMyTestService(myTestRepo repositories.MyTestRepository) *myTestService {
	return &myTestService{myTestRepo}
}

func (service *myTestService) GetMasterSQLTableList(c echo.Context) (string, error) {
	return service.myTestRepository.GetMasterSQLTableName(c)
}
