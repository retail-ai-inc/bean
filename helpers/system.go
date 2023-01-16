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
