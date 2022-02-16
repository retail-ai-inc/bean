/**#bean*/ /*#bean.replace({{ .Copyright }})**/
package helpers

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func ParseBeanSystemFilesAndDirectorires() {

	isExist, err := IsFilesExistInDirectory("repositories/", []string{"infra.go"})
	if err != nil {
		fmt.Printf("Bean `repositories/` directory parsing error: %v Server 🚀  crash landed. Exiting...\n", err)

		// Go does not use an integer return value from main to indicate exit status.
		// To exit with a non-zero status we should use os.Exit.
		os.Exit(0)
	}

	if !isExist {
		fmt.Printf("Bean files (infra.go) are not exist in `repositories/` directory. Bean 🚀  crash landed. Exiting...\n")

		// Go does not use an integer return value from main to indicate exit status.
		// To exit with a non-zero status we should use os.Exit.
		os.Exit(0)
	}

	// Check `services/` directory exist or not.
	if pathAbs, err := filepath.Abs("services/"); err == nil {
		if fileInfo, err := os.Stat(pathAbs); os.IsNotExist(err) || !fileInfo.IsDir() {
			fmt.Printf("Bean `services/` directory not exist. Please create one and set the right permissions. Bean 🚀  crash landed. Exiting...\n")

			// Go does not use an integer return value from main to indicate exit status.
			// To exit with a non-zero status we should use os.Exit.
			os.Exit(0)
		}
	}

	// Check `handlers/` directory exist or not.
	if pathAbs, err := filepath.Abs("handlers/"); err == nil {
		if fileInfo, err := os.Stat(pathAbs); os.IsNotExist(err) || !fileInfo.IsDir() {
			fmt.Printf("Bean `handlers/` directory not exist. Please create one and set the right permissions. Bean 🚀  crash landed. Exiting...\n")

			// Go does not use an integer return value from main to indicate exit status.
			// To exit with a non-zero status we should use os.Exit.
			os.Exit(0)
		}
	}

	isExist, err = IsFilesExistInDirectory("commands/", []string{"root.go", "start.go"})
	if err != nil {
		fmt.Printf("Bean `commands/` directory parsing error: %v Server 🚀  crash landed. Exiting...\n", err)

		// Go does not use an integer return value from main to indicate exit status.
		// To exit with a non-zero status we should use os.Exit.
		os.Exit(0)
	}

	if !isExist {
		fmt.Printf("Bean files (root.go or start.go) are not exist in `commands/` directory. Bean 🚀  crash landed. Exiting...\n")

		// Go does not use an integer return value from main to indicate exit status.
		// To exit with a non-zero status we should use os.Exit.
		os.Exit(0)
	}

	// Check `env.json` exist or nor.
	if _, err := os.Stat("env.json"); errors.Is(err, os.ErrNotExist) {
		fmt.Printf("Bean `env.json` does not exist. Bean 🚀  crash landed. Exiting...\n")

		// Go does not use an integer return value from main to indicate exit status.
		// To exit with a non-zero status we should use os.Exit.
		os.Exit(0)
	}
}
