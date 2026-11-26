package aes

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 32 byte key (AES-256), base64 encoded.
const testKey = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="

func TestBeanAES_RoundTrip(t *testing.T) {
	for _, plain := range []string{"", "a", "secret-password", "0123456789abcdef", "データベースのパスワード"} {
		enc, err := BeanAESEncrypt(testKey, plain)
		require.NoError(t, err, plain)
		dec, err := BeanAESDecrypt(testKey, enc)
		require.NoError(t, err, plain)
		assert.Equal(t, plain, dec)
	}
}

// Texts encrypted before the MAC was verified must still decrypt (the format is unchanged).
func TestBeanAESDecrypt_ExistingText(t *testing.T) {
	const existing = "eyJpdiI6InQzNjBkd1dzNFZMRmREV3J6MTFVTVE9PSIsIm1hYyI6Ijg0ZjJmMWU2MDQyMWYwM2Q1ZGY2ZDFkZmI0YjE2ZDBkNWZkOTM4ODAxYzdmNDNjOWExZGYwNTM1OWE0MTAxZWMiLCJ2YWx1ZSI6IklVNVNobnkzbE8rTi9CM3lCWkR3akE9PSJ9"
	dec, err := BeanAESDecrypt(testKey, existing)
	require.NoError(t, err)
	assert.Equal(t, "db-pässword", dec)
}

func decodeTicket(t *testing.T, text string) beanAESTicket {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(text)
	require.NoError(t, err)
	var ticket beanAESTicket
	require.NoError(t, json.Unmarshal(raw, &ticket))
	return ticket
}

func encodeTicket(t *testing.T, ticket beanAESTicket) string {
	t.Helper()
	raw, err := json.Marshal(ticket)
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(raw)
}

func TestBeanAESDecrypt_RejectsModifiedText(t *testing.T) {
	enc, err := BeanAESEncrypt(testKey, "secret-password")
	require.NoError(t, err)

	// Flipping one bit of the IV changes the first decrypted block (CBC malleability): it must be rejected.
	ticket := decodeTicket(t, enc)
	iv, err := base64.StdEncoding.DecodeString(ticket.IV)
	require.NoError(t, err)
	iv[0] ^= 0x01
	flipped := ticket
	flipped.IV = base64.StdEncoding.EncodeToString(iv)

	forgedMAC := ticket
	forgedMAC.MAC = "deadbeef"

	noHexMAC := ticket
	noHexMAC.MAC = "not-hex"

	otherKey, err := BeanAESEncrypt("ZmVkY2JhOTg3NjU0MzIxMGZlZGNiYTk4NzY1NDMyMTA=", "secret-password")
	require.NoError(t, err)

	for name, text := range map[string]string{
		"iv bit flipped":     encodeTicket(t, flipped),
		"forged mac":         encodeTicket(t, forgedMAC),
		"mac not hex":        encodeTicket(t, noHexMAC),
		"encrypted with key": otherKey,
	} {
		t.Run(name, func(t *testing.T) {
			dec, err := BeanAESDecrypt(testKey, text)
			assert.ErrorIs(t, err, ErrInvalidMAC)
			assert.Empty(t, dec)
		})
	}
}

// Malformed texts return an error instead of panicking, even with a valid MAC.
func TestBeanAESDecrypt_MalformedText(t *testing.T) {
	key, err := base64.StdEncoding.DecodeString(testKey)
	require.NoError(t, err)
	signed := func(iv, value string) string {
		return encodeTicket(t, beanAESTicket{IV: iv, Value: value, MAC: ComputeHmacSha256(iv+value, string(key))})
	}
	b64 := func(n int) string { return base64.StdEncoding.EncodeToString(make([]byte, n)) }

	for name, text := range map[string]string{
		"not base64":        "%%%",
		"not json":          base64.StdEncoding.EncodeToString([]byte("nope")),
		"missing iv":        base64.StdEncoding.EncodeToString([]byte(`{"value":"x","mac":"00"}`)),
		"iv wrong type":     base64.StdEncoding.EncodeToString([]byte(`{"iv":1,"value":"x","mac":"00"}`)),
		"short iv":          signed(b64(8), b64(16)),
		"value not blocks":  signed(b64(16), b64(15)),
		"value not base64":  signed(b64(16), "%%%"),
		"empty value bytes": signed(b64(16), b64(0)+"="),
	} {
		t.Run(name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				_, err := BeanAESDecrypt(testKey, text)
				assert.Error(t, err)
			})
		})
	}
}
