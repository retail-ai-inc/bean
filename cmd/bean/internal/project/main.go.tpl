{{ .Copyright }}
package main

import "{{ .PkgPath }}/commands"

func main() {
	commands.Execute()
}
