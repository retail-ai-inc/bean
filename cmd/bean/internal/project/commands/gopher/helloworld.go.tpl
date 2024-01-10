{{ .Copyright }}

// ./{{ .PkgName }} gopher helloworld

package gopher

import (
	"errors"
	"fmt"

	"github.com/retail-ai-inc/bean/v2"
	"github.com/spf13/cobra"
)

func init() {
	cmd := &cobra.Command{
		Use:   "helloworld",
		Short: "Hello world!",
		Long:  `This command will just print hello world otherwise hello mars`,
		RunE: func(cmd *cobra.Command, args []string) error {
			NewHellowWorld()
			err := helloWorld("hello")
			if err != nil {
				// If you turn on `sentry` via env.json then the error will be captured by sentry otherwise ignore.
				bean.SentryCaptureException(nil, err)
			}

			return err
		},
	}

	GopherCmd.AddCommand(cmd)
}

func NewHellowWorld() {
	// IMPORTANT: If you pass `false` then database connection will not be initialized.
	_ = initBean(false)
}

func helloWorld(h string) error {
	if h == "hello" {
		fmt.Println("hellow world")
		return nil
	}

	return errors.New("hello mars")
}
