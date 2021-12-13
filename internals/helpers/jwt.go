/*
 * Copyright The RAI Inc.
 * The RAI Authors
 */

package helpers

import (
	"errors"

	ejwt "bean/externals/jwt"

	"github.com/golang-jwt/jwt"
	"github.com/labstack/echo/v4"
	"github.com/spf13/viper"
)

/*
 * ExtractUserInfoFromJWT extracts user info from JWT. It is faster than calling redis to get those info.
 */
func ExtractUserInfoFromJWT(c echo.Context) (*ejwt.UserJWTTokenData, error) {

	tokenString := ExtractJWTFromHeader(c)
	token, err := jwt.ParseWithClaims(tokenString, &ejwt.UserJWTTokenData{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(viper.GetString("jwt.secret")), nil
	})

	if err != nil {
		return nil, errors.New("invalid user token")
	}

	if claims, ok := token.Claims.(*ejwt.UserJWTTokenData); ok && token.Valid {

		return claims, nil

	} else if ve, ok := err.(*jwt.ValidationError); ok {

		if ve.Errors&jwt.ValidationErrorMalformed != 0 {
			return nil, errors.New("invalid user token")

		} else if ve.Errors&(jwt.ValidationErrorExpired|jwt.ValidationErrorNotValidYet) != 0 {
			return nil, errors.New("token is expired")

		} else {
			return nil, errors.New("invalid user token")
		}

	} else {
		return nil, errors.New("invalid user token")
	}
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
