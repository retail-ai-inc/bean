/*
 * Copyright The RAI Inc.
 * The RAI Authors
 */

package cli

import (
	"fmt"
	"os"

	"bean/internals/chalk"
	"bean/internals/global"
	"bean/internals/sentry"

	"github.com/labstack/echo/v4"
)

/*
 * Write the error to console or sentry when a `gopher` command panics.
 */
func RecoverPanic(c echo.Context) {

	if r := recover(); r != nil {

		err, ok := r.(error)
		if !ok {
			err = fmt.Errorf("%v", r)
		}

		// Run this function synchronously to release the `context` properly.
		sentry.PushData(c, err, nil, false)
	}

	// Release the acquired context. `c` is a global context here.
	// A single `gopher` command is using only one context in it's life time.
	global.EchoInstance.ReleaseContext(c)

	// Exit the `gopher` command.
	os.Exit(0)
}

func DisplayHelpMessage(commandDescription map[string]string) {

	helpMessage := `Bean 1.0
	
` + chalk.Bold.TextStyle("NAME:") + `
	bean gopher - This is a CLI and make utility for bean to do awesome stuff

` + chalk.Bold.TextStyle("USAGE:") + `
	bean gopher command[:options] [arguments...]
	or
	make -- command[:options] [arguments...]

` + chalk.Bold.TextStyle("NOTES:") + `
	` + chalk.Bold.TextStyle("make") + ` supporting all ` + chalk.Bold.TextStyle("bean gopher") + ` command. Additionally, it's also supporting following:
	start, stop, reload, restart, build, clean, install, run, get, all, test, debug commands.
	
` + chalk.Bold.TextStyle("COMMANDS:") + `
	help			Shows a list of commands or help for one command
	route:list		List all registered routes

` + chalk.Bold.TextStyle("ADDITIONAL MAKE COMMANDS:") + `
	all			Run ` + chalk.Bold.TextStyle("build") + ` and ` + chalk.Bold.TextStyle("test") + ` command
	build			Compile and install with race detection and place the file in /bin folder
	clean			Removes object files from package source directories and /bin folder
	debug			Start delve debug server (only for local env)
	get			Get and adds dependencies to the current development module and then builds and installs them
	install			Compile and install without race detection and place the file in /bin folder
	lint			Code linters aggregator
	run			Run the server in terminal
	restart			Restart the 
	reload			Reload the config file
	start			Start the server
	stop			Stop the server gracefully
`
	for k, v := range commandDescription {
		helpMessage = helpMessage + k + `\t\t\t` + v + `\n`
	}

	fmt.Print(helpMessage)
}
