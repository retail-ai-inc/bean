// MIT License

// Copyright (c) The RAI Authors

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
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
