/**#bean*/ /*#bean.replace({{ .Copyright }})**/
package helpers

import (
	"errors"

	"github.com/golang-jwt/jwt"
	"github.com/labstack/echo/v4"
)

var (
	errorMessageInvalidToken = "token is invalid"
	errorMessageExpiredToken = "token is expired"
)

// ExtractUserInfoFromJWT extracts user info from JWT. It is faster than calling redis to get those info.
func DecodeJWT(c echo.Context, claims jwt.Claims, secret string) error {

	tokenString := ExtractJWTFromHeader(c)
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})

	if err != nil {

		if ve, ok := err.(*jwt.ValidationError); ok {

			if ve.Errors&jwt.ValidationErrorMalformed != 0 {
				return errors.New(errorMessageInvalidToken)

			} else if ve.Errors&(jwt.ValidationErrorExpired|jwt.ValidationErrorNotValidYet) != 0 {
				return errors.New(errorMessageExpiredToken)

			} else {
				return errors.New(errorMessageInvalidToken)
			}
		}

		return errors.New(errorMessageInvalidToken)
	}

	if !token.Valid {

		return errors.New(errorMessageInvalidToken)
	}

	return nil
}

// ExtractJWTFromHeader returns the JWT token string from authorization header.
func ExtractJWTFromHeader(c echo.Context) string {

	var tokenString string

	authHeader := c.Request().Header.Get("Authorization")

	l := len("Bearer")

	if len(authHeader) > l+1 && authHeader[:l] == "Bearer" {
		tokenString = authHeader[l+1:]
	} else {
		tokenString = ""
	}

	return tokenString
}

func EncodeJWT(claims jwt.Claims, secret string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Generate encoded token and send it as response.
	return token.SignedString([]byte(secret))
}
