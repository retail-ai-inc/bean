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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

type jwtData struct {
	Name    string
	Age     uint
	Hobbies []string
	jwt.StandardClaims
}

func Test_DecodeJWTWithJsonUnmarshalStyle(t *testing.T) {
	e := echo.New()
	c := e.AcquireContext()
	defer e.ReleaseContext(c)

	jwtSecret := "123456"

	data := &jwtData{
		Name:    "raicart",
		Age:     uint(18),
		Hobbies: []string{"football", "basketball"},
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(6000 * time.Second).Unix(),
		},
	}
	// Create token with claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, data)

	// Generate encoded token and send it as response.
	tokenString, err := token.SignedString([]byte(jwtSecret))
	assert.NoError(t, err)

	extractedData := new(jwtData)
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

	extractedData := new(jwtData)
	err := DecodeJWT(c, extractedData, "testSecret")
	assert.Equal(t, "token is invalid", err.Error())
}

func Test_DecodeJWTWhenExpiredToken(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	data := &jwtData{
		Name:    "raicart",
		Age:     uint(18),
		Hobbies: []string{"football", "basketball"},
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(1 * time.Second).Unix(),
		},
	}
	token, err := EncodeJWT(data, "testSecret")
	assert.NoError(t, err)

	c.Request().Header.Set("Authorization", "Bearer "+token)

	time.Sleep(2 * time.Second)
	extractedData := new(jwtData)
	err = DecodeJWT(c, extractedData, "testSecret")
	assert.Equal(t, "token is expired", err.Error())
}
