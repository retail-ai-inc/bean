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

package aes

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

// This function will encrypt a text using AES in base64. The `key` a.k.a passphrase is also encoded using base64.
func BeanAESEncrypt(key, plainText string) (string, error) {

	keyInByte, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return "", err
	}

	plainTextByte := []byte(plainText)
	plainTextByte, err = Pkcs7Pad(plainTextByte, aes.BlockSize)
	if err != nil {
		return "", err
	}

	if len(plainTextByte)%aes.BlockSize != 0 {
		err := fmt.Errorf(`plainText: "%s" has the wrong block size`, plainTextByte)
		return "", err
	}

	iv := make([]byte, 16)
	_, err = rand.Read(iv)
	if err != nil {
		return "", err
	}

	cipherBlock, err := aes.NewCipher(keyInByte)
	if err != nil {
		return "", err
	}

	encryptedTextByte := make([]byte, len(plainTextByte))

	mode := cipher.NewCBCEncrypter(cipherBlock, iv)
	mode.CryptBlocks(encryptedTextByte, plainTextByte)

	encryptedTextBase64 := base64.StdEncoding.EncodeToString(encryptedTextByte)
	ivTextBase64 := base64.StdEncoding.EncodeToString(iv)

	data := ivTextBase64 + encryptedTextBase64
	mac := ComputeHmacSha256(data, string(keyInByte))

	resTicket, err := json.Marshal(beanAESTicket{IV: ivTextBase64, Value: encryptedTextBase64, MAC: mac})
	if err != nil {
		return "", err
	}

	ticketR := base64.StdEncoding.EncodeToString(resTicket)

	return ticketR, nil
}

// ErrInvalidMAC is returned by `BeanAESDecrypt` when the MAC of the encrypted text does not match: the text was
// not encrypted with this key, or it was modified.
var ErrInvalidMAC = errors.New("aes: invalid MAC")

// beanAESTicket is the JSON payload of an encrypted text (the same format as Laravel's `Encrypter`).
type beanAESTicket struct {
	IV    string `json:"iv"`
	MAC   string `json:"mac"`
	Value string `json:"value"`
}

// This function will decrypt an AES encrypted text in base64. The `key` a.k.a passphrase is also encoded using base64.
//
// The MAC is verified (in constant time) before decrypting, so a modified or forged text is rejected with
// `ErrInvalidMAC` instead of being decrypted. A malformed text returns an error.
func BeanAESDecrypt(key, encryptedText string) (string, error) {

	keyInByte, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return "", err
	}

	base64DecodedByteText, err := base64.StdEncoding.DecodeString(encryptedText)
	if err != nil {
		return "", err
	}

	var ticket beanAESTicket
	if err := json.Unmarshal(base64DecodedByteText, &ticket); err != nil {
		return "", err
	}
	if ticket.IV == "" || ticket.Value == "" || ticket.MAC == "" {
		return "", errors.New("aes: invalid encrypted text: iv, value and mac are required")
	}

	// Encrypt-then-MAC: authenticate before touching the ciphertext.
	gotMAC, err := hex.DecodeString(ticket.MAC)
	if err != nil {
		return "", ErrInvalidMAC
	}
	if !hmac.Equal(gotMAC, computeHmacSha256(ticket.IV+ticket.Value, keyInByte)) {
		return "", ErrInvalidMAC
	}

	iv, err := base64.StdEncoding.DecodeString(ticket.IV)
	if err != nil {
		return "", err
	}
	if len(iv) != aes.BlockSize {
		return "", fmt.Errorf("aes: invalid iv length %d", len(iv))
	}

	encryptedValue, err := base64.StdEncoding.DecodeString(ticket.Value)
	if err != nil {
		return "", err
	}
	if len(encryptedValue) == 0 || len(encryptedValue)%aes.BlockSize != 0 {
		return "", fmt.Errorf("aes: invalid encrypted value length %d", len(encryptedValue))
	}

	cipherBlock, err := aes.NewCipher(keyInByte)
	if err != nil {
		return "", err
	}

	cipher.NewCBCDecrypter(cipherBlock, iv).CryptBlocks(encryptedValue, encryptedValue)

	out, err := Pkcs7Unpad(encryptedValue, aes.BlockSize)
	if err != nil {
		return "", err
	}

	return string(out), nil
}

// Pkcs7Pad right-pads the given byte slice with 1 to n bytes, where
// n is the block size. The size of the result is x times n, where x
// is at least 1 (empty data is padded to one full block).
func Pkcs7Pad(data []byte, blocklen int) ([]byte, error) {

	if blocklen <= 0 {
		return nil, fmt.Errorf("invalid blocklen %d", blocklen)
	}

	n := blocklen - (len(data) % blocklen)
	pb := make([]byte, len(data)+n)
	copy(pb, data)
	copy(pb[len(data):], bytes.Repeat([]byte{byte(n)}, n))

	return pb, nil
}

// Pkcs7Unpad validates and unpads data from the given bytes slice.
// The returned value will be 1 to n bytes smaller depending on the
// amount of padding, where n is the block size.
func Pkcs7Unpad(data []byte, blocklen int) ([]byte, error) {

	if blocklen <= 0 {
		return nil, fmt.Errorf("invalid blocklen %d", blocklen)
	}

	if len(data)%blocklen != 0 || len(data) == 0 {
		return nil, fmt.Errorf("invalid data len %d", len(data))
	}

	padlen := int(data[len(data)-1])
	if padlen > blocklen || padlen == 0 {
		return nil, fmt.Errorf("invalid padding")
	}

	pad := data[len(data)-padlen:]
	for i := 0; i < padlen; i++ {
		if pad[i] != byte(padlen) {
			return nil, fmt.Errorf("invalid padding")
		}
	}

	return data[:len(data)-padlen], nil
}

// ComputeHmacSha256 returns the hex encoded HMAC-SHA256 of message.
func ComputeHmacSha256(message string, secret string) string {
	return hex.EncodeToString(computeHmacSha256(message, []byte(secret)))
}

func computeHmacSha256(message string, key []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(message))
	return h.Sum(nil)
}
