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
	"crypto/md5"
	"encoding/hex"

	"github.com/google/uuid"
)

// DeterministicUUID generates a deterministic UUID based on the provided seeder string.
// It uses the MD5 hash of the seeder as the input to create the UUID.
// The first 16 bytes of the MD5 hash are used to generate the UUID, ensuring that the same
// seeder always produces the same UUID.
func DeterministicUUID(seeder string) (string, error) {

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
