/**#bean*/ /*#bean.replace({{ .Copyright }})**/
package main

import /**#bean*/ "demo/commands" /*#bean.replace("{{ .PkgName }}/commands")**/

func main() {
	commands.Execute()
}
