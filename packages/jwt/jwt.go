/*
 * Copyright The RAI Inc.
 * The RAI Authors
 */

package jwt

import (
	"github.com/golang-jwt/jwt"
)

// UserJWTTokenData Stores the user information
type UserJWTTokenData struct {
	ID uint64
	/* Add your own data here */
	jwt.StandardClaims
}
