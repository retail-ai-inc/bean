/*
 * Copyright The RAI Inc.
 * The RAI Authors
 */

package handlers

import (
	"bean/interfaces"
	"net/http"

	"github.com/labstack/echo/v4"
)

func NewMyTestHandler(myTestSvc interfaces.MyTestService) *MyTestHandler {
	return &MyTestHandler{myTestSvc}
}

func (handler *MyTestHandler) MyTestIndex(c echo.Context) error {

	res, _ := handler.MyTestService.GetMasterSQLTableList(c)

	c.Logger().Info(res["dbName"])

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Howdy! I am bean 🚀 ",
	})
}
