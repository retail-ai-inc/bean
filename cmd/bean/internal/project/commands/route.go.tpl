{{ .Copyright }}
package commands

import (
	"os"
	"sort"
	"strings"

	"{{ .PkgPath }}/routers"

	"github.com/olekukonko/tablewriter"
	"github.com/retail-ai-inc/bean"
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

func init() {
	routeCmd.AddCommand(listCmd)
	rootCmd.AddCommand(routeCmd)
}

func routeList(cmd *cobra.Command, args []string) {
	// Create a bean object
	b := bean.New()

	// Create an empty database dependency.
	b.DBConn = &bean.DBDeps{}

	// Init different routes.
	routers.Init(b)

	// Consider the allowed methods to display only URI path that's support it.
	allowedMethod := bean.BeanConfig.HTTP.AllowedMethod

	table := tablewriter.NewWriter(os.Stdout)
	header := []string{"Path", "Method", "Handler"}
	table.SetHeader(header)

	for _, r := range b.Echo.Routes() {

		if strings.Contains(r.Name, "glob..func1") {
			continue
		}

		// XXX: IMPORTANT - `allowedMethod` has to be a sorted slice.
		i := sort.SearchStrings(allowedMethod, r.Method)

		if i >= len(allowedMethod) || allowedMethod[i] != r.Method {
			continue
		}

		row := []string{r.Path, r.Method, strings.TrimRight(r.Name, "-fm")}
		table.Append(row)
	}

	table.Render()
}
