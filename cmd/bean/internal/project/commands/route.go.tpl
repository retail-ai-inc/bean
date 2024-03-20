{{ .Copyright }}
package commands

import (
    "encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"{{ .PkgPath }}/routers"
	
	"github.com/getsentry/sentry-go"
	"github.com/olekukonko/tablewriter"
	"github.com/retail-ai-inc/bean/v2"
	"github.com/spf13/cobra"
)

var (
	// routeCmd represents the `route` command.
	routeCmd = &cobra.Command{
		Use:   "route [command]",
		Short: "This command requires a sub command parameter.",
		Long:  "",
	}
)

var (
	// listCmd represents the route list command.
	listCmd = &cobra.Command{
		Use:   "list",
		Short: "Display the route list.",
		Long:  `This command will display all the URI listed in route.go file.`,
		Run:   routeList,
	}
)

var isJsonOutput bool

func init() {
	routeCmd.AddCommand(listCmd)
	listCmd.Flags().BoolVarP(&isJsonOutput, "json", "j", false, "will ouput the result in json")
	rootCmd.AddCommand(routeCmd)
}

func routeList(cmd *cobra.Command, args []string) {
	// Just initialize a plain sentry client option structure if sentry is on.
	if bean.BeanConfig.Sentry.On {
		bean.BeanConfig.Sentry.ClientOptions = &sentry.ClientOptions{
			Debug:       false,
			Dsn:         bean.BeanConfig.Sentry.Dsn,
			Environment: bean.BeanConfig.Environment,
		}
	}

	// Create a bean object
	b := bean.New()

	// Create an empty database dependency.
	b.DBConn = &bean.DBDeps{}

	// Init different routes.
	routers.Init(b)

	// Consider the allowed methods to display only URI path that's support it.
	allowedMethod := bean.BeanConfig.HTTP.AllowedMethod

	jsonOutput := make([]map[string]interface{}, 0)

	table := tablewriter.NewWriter(os.Stdout)
	header := []string{"Path", "Method", "Handler"}
	table.SetHeader(header)

	for _, r := range b.Echo.Routes() {

		// Do not output internal library-related handler info
		if strings.Contains(r.Name, "glob..func1") || strings.Contains(r.Name, ".init.func1") {
			continue
		}

		// XXX: IMPORTANT - `allowedMethod` has to be a sorted slice.
		i := sort.SearchStrings(allowedMethod, r.Method)

		if i >= len(allowedMethod) || allowedMethod[i] != r.Method {
			continue
		}

		row := []string{r.Path, r.Method, strings.TrimSuffix(r.Name, "-fm")}
		jsonOutput = append(jsonOutput, map[string]interface{}{
			"path":   r.Path,
			"method": r.Method,
			"name":   strings.TrimSuffix(r.Name, "-fm"),
		})

		table.Append(row)
	}

	if isJsonOutput {
		jsonInByte, _ := json.Marshal(jsonOutput)
		fmt.Println(string(jsonInByte))
	} else {
		table.Render()
	}
}
