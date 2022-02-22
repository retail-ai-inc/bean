{{ .Copyright }})
package commands

import (
	"fmt"
	"os"

	str "github.com/retail-ai-inc/bean/string"
	"github.com/spf13/cobra"
)

var (
	// gen represents the `gen` command.
	genCmd = &cobra.Command{
		Use:   "gen [command]",
		Short: "Generate a resource or key.",
		Long:  `This command requires a sub command parameter to generate a resource like a file or server or a key.`,
	}
)

var (
	// genSecretCmd represents the `gen secret` command.
	genSecretCmd = &cobra.Command{
		Use:   "secret",
		Short: "Generate 32 character long random alphanumeric string to use as secret.",
		Long:  `This alphanumeric random string is useful to use as secret in env.json file.`,
		Args:  cobra.ExactArgs(0),
		Run:   genSecret,
	}
)

func init() {

	genCmd.AddCommand(genSecretCmd)
	rootCmd.AddCommand(genCmd)
}

func genSecret(cmd *cobra.Command, args []string) {

	// Generate a 32 character long random secret string.
	secret, err := str.GenerateRandomString(32, false)
	if err != nil {
		fmt.Println(err)
		os.Exit(0)
	}

	fmt.Println(secret)
}
