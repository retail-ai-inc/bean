{{ .Copyright }}
package commands

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/retail-ai-inc/bean/framework/aes"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// aesEncryptCmd represents the aes:encrypt command.
	aesEncryptCmd = &cobra.Command{
		Use:   "aes:encrypt",
		Short: "Encrypt a plaintext using AES algorithm",
		Long: `This command will encrypt a text using AES algo and print it as a base64 string. The secret a.k.a passphrase is also encoded 
		as a base64 string in env.json file.`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return errors.New("requires a plain text to encrypt")
			}

			return nil
		},
		Run: aesEncrypt,
	}
)

var (
	// aesDecryptCmd represents the aes:decrypt command.
	aesDecryptCmd = &cobra.Command{
		Use:   "aes:decrypt",
		Short: "Decrypt a base64 encrypted text using AES algorithm",
		Long: `This command will decrypt an encrypted base64 text using AES algo and print it as a plain text. The secret a.k.a passphrase is also encoded 
		as a base64 string in env.json file.`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return errors.New("requires an encrypted text to decrypt")
			}

			return nil
		},
		Run: aesDecrypt,
	}
)

func init() {
	rootCmd.AddCommand(aesEncryptCmd)
	rootCmd.AddCommand(aesDecryptCmd)
}

func aesEncrypt(cmd *cobra.Command, args []string) {

	plaintext := strings.Join(args, "")

	secret := viper.GetString("secret")
	encryptedText, err := aes.BeanAESEncrypt(secret, plaintext)
	if err != nil {
		fmt.Println(err)
		os.Exit(0)
	}

	fmt.Println(encryptedText)
}

func aesDecrypt(cmd *cobra.Command, args []string) {

	secret := viper.GetString("secret")
	decryptedText, err := aes.BeanAESDecrypt(secret, args[0])
	if err != nil {
		fmt.Println(err)
		os.Exit(0)
	}

	fmt.Println(decryptedText)
}
