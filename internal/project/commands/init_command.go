/**#bean*/ /*#bean.replace({{ .Copyright }})**/
// This is the base file for all our CLI commands. Please handle with care.
package gopher

import (
	/**#bean*/
	"demo/framework/internals/cli" /*#bean.replace("{{ .PkgPath }}/framework/internals/cli")**/
	/**#bean*/
	"demo/framework/internals/helpers" /*#bean.replace("{{ .PkgPath }}/framework/internals/helpers")**/
	"os"

	"github.com/labstack/echo/v4"
)

// These are common singleton variables to hold the `echo` instance, context and
// our running environment (local, staging and production).
var c echo.Context
var e *echo.Echo
var environment string

// This is a global variable which will hold command `name` - `description` pair.
var commandDescription map[string]string

func init() {

	if len(os.Args) >= 2 {

		switch os.Args[1] {

		case "gopher":
			gopherSubcommand()
		}
	}
}

func gopherSubcommand() {

	// XXX: IMPORTANT - Recover panic properly for a gopher command. This will also do the final exit of the command.
	defer cli.RecoverPanic(c)

	// XXX: IMPORTANT - Initialize the CLI environment with proper config json file and get echo instance, context and environment string.
	e, c, environment = helpers.GetContextInstanceEnvironmentAndConfig()

	c.Logger().Info("ENVIRONMENT: ", environment)

	if len(os.Args) >= 3 {

		switch os.Args[2] {

		/* ADD YOUR OWN CASE AND COMMANDS */

		case "help":
			cli.DisplayHelpMessage(commandDescription)

		default:
			cli.DisplayHelpMessage(commandDescription)
			os.Exit(0)
		}

	} else {

		cli.DisplayHelpMessage(commandDescription)
		os.Exit(0)
	}
}
