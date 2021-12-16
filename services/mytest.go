/*
 * Copyright The RAI Inc.
 * The RAI Authors
 */

package services

import (
	"bean/repositories"

	"github.com/labstack/echo/v4"
)

type MyTestService interface {
	GetMasterSQLTableList(c echo.Context) (map[string]interface{}, error)
}

type myTestService struct {
	myTestRepository repositories.MyTestRepository
}

func NewMyTestService(myTestRepo repositories.MyTestRepository) *myTestService {
	return &myTestService{myTestRepo}
}

func (service *myTestService) GetMasterSQLTableList(c echo.Context) (map[string]interface{}, error) {

	return service.myTestRepository.GetMasterSQLTableList(c)
}
