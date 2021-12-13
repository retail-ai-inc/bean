/*
 * Copyright The RAI Inc.
 * The RAI Authors
 */

package services

import (
	"bean/interfaces"

	"github.com/labstack/echo/v4"
)

func NewMyTestService(myTestRepo interfaces.MyTestRepository) *MyTestService {
	return &MyTestService{myTestRepo}
}

func (service *MyTestService) GetMasterSQLTableList(c echo.Context) (map[string]interface{}, error) {

	return service.MyTestRepository.GetMasterSQLTableList(c)
}
