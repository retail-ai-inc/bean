package helpers

import (
	"crypto/md5"
	"encoding/hex"

	"github.com/google/uuid"
)

func DeterministicUUID(seeder string) (string, error) {
	// DeterministicUUID generates a deterministic UUID based on the provided seeder string.
	// It uses the MD5 hash of the seeder as the input to create the UUID.
	// The first 16 bytes of the MD5 hash are used to generate the UUID, ensuring that the same
	// seeder always produces the same UUID.

	// calculate the MD5 hash of the seeder reference
	md5hash := md5.New()
	md5hash.Write([]byte(seeder))

	// convert the hash value to a string
	md5string := hex.EncodeToString(md5hash.Sum(nil))

	// generate the UUID from the first 16 bytes of the MD5 hash
	uuid, err := uuid.FromBytes([]byte(md5string[0:16]))
	if err != nil {
		return "", err
	}

	return uuid.String(), nil
}
