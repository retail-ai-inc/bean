/*
 * Copyright 2021 The RAI Inc.
 * The RAI Authors
 */

package jwt_test

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func Test_jwt(t *testing.T) {
	e := echo.New()
	c := e.AcquireContext()
	defer e.ReleaseContext(c)

	type JWTData struct {
		Name    string
		Age     uint
		Hobbies []string
		jwt.StandardClaims
	}

	jwtSecret := "123456"

	data := &JWTData{
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

	extractedData := new(JWTData)
	token, err = jwt.ParseWithClaims(tokenString, extractedData, func(token *jwt.Token) (interface{}, error) {
		return []byte(jwtSecret), nil
	})
	assert.NoError(t, err)

	assert.Equal(t, *data, *extractedData)
	assert.Equal(t, extractedData, token.Claims)
}
