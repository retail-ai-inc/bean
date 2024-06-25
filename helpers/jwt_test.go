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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

type customClaims struct {
	Name    string
	Age     uint
	Hobbies []string
	jwt.RegisteredClaims
}

var _ jwt.Claims = &customClaims{}

// Validate validates the custom claims based on application-specific logic.
// OPTIONAL: Implement jwt.ClaimsValidator interface if you want to validate the claims.
func (c *customClaims) Validate() error {
	if c.Age < 18 {
		return errors.New("age is less than 18")
	}
	return nil
}

var _ jwt.ClaimsValidator = &customClaims{}

func Test_DecodeJWTWithJsonUnmarshalStyle(t *testing.T) {
	e := echo.New()
	c := e.AcquireContext()
	defer e.ReleaseContext(c)

	jwtSecret := "123456"

	data := &customClaims{
		Name:    "raicart",
		Age:     uint(18),
		Hobbies: []string{"football", "basketball"},
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(6000 * time.Second)),
		},
	}
	// Create token with claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, data)

	// Generate encoded token and send it as response.
	tokenString, err := token.SignedString([]byte(jwtSecret))
	assert.NoError(t, err)

	extractedData := new(customClaims)
	token, err = jwt.ParseWithClaims(tokenString, extractedData, func(token *jwt.Token) (interface{}, error) {
		return []byte(jwtSecret), nil
	})
	assert.NoError(t, err)

	assert.Equal(t, *data, *extractedData)
	assert.Equal(t, extractedData, token.Claims)
}

func Test_DecodeJWTWhenInvalidToken(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Request().Header.Set("Authorization", "Bearer "+"token")

	extractedData := new(customClaims)
	err := DecodeJWT(c, extractedData, "testSecret")
	assert.True(t, errors.Is(err, ErrJWTokenInvalid))
}

func Test_DecodeJWTWhenExpiredToken(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	data := &customClaims{
		Name:    "raicart",
		Age:     uint(18),
		Hobbies: []string{"football", "basketball"},
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Second)),
		},
	}
	token, err := EncodeJWT(data, "testSecret")
	assert.NoError(t, err)

	c.Request().Header.Set("Authorization", "Bearer "+token)

	time.Sleep(2 * time.Second)
	extractedData := new(customClaims)
	err = DecodeJWT(c, extractedData, "testSecret")
	assert.True(t, errors.Is(err, ErrJWTokenExpired))
}
