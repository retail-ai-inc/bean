{{ .Copyright }}
package jwt

import (
	"github.com/golang-jwt/jwt/v4"
)

// UserJWTTokenData Stores the user information
type UserJWTTokenData struct {
	ID uint64
	/* Add your own data here */
	jwt.RegisteredClaims
}
