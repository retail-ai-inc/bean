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
	"fmt"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

var (
	ErrJWTokenInvalid = errors.New("jwt token is invalid")
	ErrJWTokenExpired = errors.New("jwt token is expired")
)

// EncodeJWT will encode JWT `claims` using a secret string and return a signed token as string.
func EncodeJWT(claims jwt.Claims, secret string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims) // Use HS256 algorithm.

	// Generate encoded token and send it as response.
	return token.SignedString([]byte(secret))
}

// DecodeJWT will decode JWT string into `claims` structure using a secret string.
func DecodeJWT(c echo.Context, claims jwt.Claims, secret string, opts ...jwt.ParserOption) error {

	if len(opts) == 0 {
		// Add default options if not provided.
		opts = append(opts,
			jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
			jwt.WithExpirationRequired(),
			jwt.WithStrictDecoding(),
		)
	}

	tokenString := ExtractJWTFromHeader(c)
	_, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	}, opts...)

	if err != nil {

		if errors.Is(err, jwt.ErrTokenMalformed) {
			return fmt.Errorf("%w: %w", ErrJWTokenInvalid, err)
		}

		if errors.Is(err, jwt.ErrTokenExpired) || errors.Is(err, jwt.ErrTokenNotValidYet) {
			return fmt.Errorf("%w: %w", ErrJWTokenExpired, err)
		}

		return fmt.Errorf("%w: %w", ErrJWTokenInvalid, err)
	}

	return nil
}

// ExtractJWTFromHeader will extract JWT from `Authorization` HTTP header and returns as string.
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
