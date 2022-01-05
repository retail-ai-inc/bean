/**#bean*/ /*#bean.replace({{ .Copyright }})**/
package helpers

import (
	"github.com/labstack/echo/v4"
)

func JsonLogFormat() string {

	var logFormat string

	logFormat += `{`
	logFormat += `"time": "${time_rfc3339_nano}", `
	logFormat += `"level": "ACCESS", `
	logFormat += `"id": "${id}", `
	logFormat += `"remote_ip": "${remote_ip}", `
	logFormat += `"x-forwarded-for": "${header:x-forwarded-for}", `
	logFormat += `"host": "${host}", `
	logFormat += `"method": "${method}", `
	logFormat += `"uri": "${uri}", `
	logFormat += `"user_agent": "${user_agent}", `
	logFormat += `"status": ${status}, `
	logFormat += `"error": "${error}", `
	logFormat += `"latency": ${latency}, `
	logFormat += `"latency_human": "${latency_human}", `
	logFormat += `"bytes_in": ${bytes_in}, `
	logFormat += `"bytes_out": ${bytes_out}`
	logFormat += "}\n"

	return logFormat
}

func BodyDumpHandler(c echo.Context, reqBody, resBody []byte) {

	c.Logger().Info("Request Path: ", c.Path(), " | Request Body: ", string(reqBody))
	c.Logger().Info("Response Body: ", string(resBody))
}
