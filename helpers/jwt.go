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
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v5"
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

// DecodeJWT will decode the JWT of the `Authorization: Bearer` header into `claims` using a secret string.
//
// The token must be signed with HS256 and have an expiration time (`exp`), and is decoded strictly. These defaults
// always apply: opts are added after them, e.g. `jwt.WithAudience(...)`, or `jwt.WithValidMethods(...)` to accept
// other algorithms. To accept tokens without `exp`, use `jwt.ParseWithClaims` directly.
func DecodeJWT(c *echo.Context, claims jwt.Claims, secret string, opts ...jwt.ParserOption) error {

	opts = append([]jwt.ParserOption{
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithStrictDecoding(),
	}, opts...)

	tokenString := ExtractJWTFromHeader(c)
	_, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	}, opts...)

	if err != nil {

		if errors.Is(err, jwt.ErrTokenMalformed) {
			return errors.Join(ErrJWTokenInvalid, err)
		}

		if errors.Is(err, jwt.ErrTokenExpired) || errors.Is(err, jwt.ErrTokenNotValidYet) {
			return errors.Join(ErrJWTokenExpired, err)
		}

		return errors.Join(ErrJWTokenInvalid, err)
	}

	return nil
}

// ExtractJWTFromHeader returns the token of an `Authorization: Bearer <token>` HTTP header, or an empty string if
// the header is missing or uses another scheme. The scheme is case-insensitive (RFC 7235).
func ExtractJWTFromHeader(c *echo.Context) string {
	scheme, token, ok := strings.Cut(c.Request().Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}
