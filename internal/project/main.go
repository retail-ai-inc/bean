/**#bean*/ /*#bean.replace({{ .Copyright }})**/
package main

import (
	/**#bean*/
	"demo/cmd"
	/*#bean.replace("{{ .PkgName }}/cmd")**/)

func main() {
	cmd.Execute()
}
