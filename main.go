/*
 * Copyright The RAI Inc.
 * The RAI Authors
 */

package main

import (
	_ "bean/commands"
	"bean/framework/bootstrap"
	"net/http"

	"github.com/spf13/viper"
)

func main() {

	// Create a new echo instance
	e := bootstrap.New()

	projectName := viper.GetString("name")

	e.Logger.Info(`Starting ` + projectName + ` server...🚀`)

	listenAt := viper.GetString("http.host") + ":" + viper.GetString("http.port")

	// Start the server
	if err := e.Start(listenAt); err != nil && err != http.ErrServerClosed {
		e.Logger.Fatal(err)
	}
}
