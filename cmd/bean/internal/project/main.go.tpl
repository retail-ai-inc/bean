{{ .Copyright }}
package main

import "{{ .PkgName }}/commands"

func main() {
	commands.Execute()
}
