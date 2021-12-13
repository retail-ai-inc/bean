/*
 * Copyright The RAI Inc.
 * The RAI Authors
 */

package helpers

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func ParseBeanSystemFilesAndDirectorires() {

	isExist, err := IsFilesExistInDirectory("interfaces/", []string{"handler.go", "repository.go", "service.go"})
	if err != nil {
		fmt.Printf("Bean `interfaces/` directory parsing error: %v Server 🚀  crash landed. Exiting...\n", err)

		// Go does not use an integer return value from main to indicate exit status.
		// To exit with a non-zero status we should use os.Exit.
		os.Exit(0)
	}

	if !isExist {
		fmt.Printf("Bean files (handler.go, repository.go and service.go) are not exist in `interfaces/` directory. Bean 🚀  crash landed. Exiting...\n")

		// Go does not use an integer return value from main to indicate exit status.
		// To exit with a non-zero status we should use os.Exit.
		os.Exit(0)
	}

	isExist, err = IsFilesExistInDirectory("repositories/", []string{"init_repository.go"})
	if err != nil {
		fmt.Printf("Bean `repositories/` directory parsing error: %v Server 🚀  crash landed. Exiting...\n", err)

		// Go does not use an integer return value from main to indicate exit status.
		// To exit with a non-zero status we should use os.Exit.
		os.Exit(0)
	}

	if !isExist {
		fmt.Printf("Bean files (init_repository.go) are not exist in `repositories/` directory. Bean 🚀  crash landed. Exiting...\n")

		// Go does not use an integer return value from main to indicate exit status.
		// To exit with a non-zero status we should use os.Exit.
		os.Exit(0)
	}

	isExist, err = IsFilesExistInDirectory("services/", []string{"init_service.go"})
	if err != nil {
		fmt.Printf("Bean `services/` directory parsing error: %v Server 🚀  crash landed. Exiting...\n", err)

		// Go does not use an integer return value from main to indicate exit status.
		// To exit with a non-zero status we should use os.Exit.
		os.Exit(0)
	}

	if !isExist {
		fmt.Printf("Bean files (init_service.go) are not exist in `services/` directory. Bean 🚀  crash landed. Exiting...\n")

		// Go does not use an integer return value from main to indicate exit status.
		// To exit with a non-zero status we should use os.Exit.
		os.Exit(0)
	}

	isExist, err = IsFilesExistInDirectory("handlers/", []string{"init_handler.go"})
	if err != nil {
		fmt.Printf("Bean `handlers/` directory parsing error: %v Server 🚀  crash landed. Exiting...\n", err)

		// Go does not use an integer return value from main to indicate exit status.
		// To exit with a non-zero status we should use os.Exit.
		os.Exit(0)
	}

	if !isExist {
		fmt.Printf("Bean files (init_handler.go) are not exist in `handlers/` directory. Bean 🚀  crash landed. Exiting...\n")

		// Go does not use an integer return value from main to indicate exit status.
		// To exit with a non-zero status we should use os.Exit.
		os.Exit(0)
	}

	isExist, err = IsFilesExistInDirectory("commands/", []string{"init_command.go"})
	if err != nil {
		fmt.Printf("Bean `commands/` directory parsing error: %v Server 🚀  crash landed. Exiting...\n", err)

		// Go does not use an integer return value from main to indicate exit status.
		// To exit with a non-zero status we should use os.Exit.
		os.Exit(0)
	}

	if !isExist {
		fmt.Printf("Bean files (init_commands.go) are not exist in `commands/` directory. Bean 🚀  crash landed. Exiting...\n")

		// Go does not use an integer return value from main to indicate exit status.
		// To exit with a non-zero status we should use os.Exit.
		os.Exit(0)
	}

	// Check `logs/` directory exist or not.
	if pathAbs, err := filepath.Abs("logs/"); err == nil {
		if fileInfo, err := os.Stat(pathAbs); os.IsNotExist(err) || !fileInfo.IsDir() {
			fmt.Printf("Bean `logs/` directory not exist. Please create one and set the right permissions. Bean 🚀  crash landed. Exiting...\n")

			// Go does not use an integer return value from main to indicate exit status.
			// To exit with a non-zero status we should use os.Exit.
			os.Exit(0)
		}
	}

	// Check `env.json` exist or nor.
	if _, err := os.Stat("env.json"); errors.Is(err, os.ErrNotExist) {
		fmt.Printf("Bean `env.json` does not exist. Bean 🚀  crash landed. Exiting...\n")

		// Go does not use an integer return value from main to indicate exit status.
		// To exit with a non-zero status we should use os.Exit.
		os.Exit(0)
	}
}
