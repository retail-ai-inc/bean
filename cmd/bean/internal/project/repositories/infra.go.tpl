{{ .Copyright }}
package repositories

import "github.com/retail-ai-inc/bean"

// IMPORTANT: DO NOT DELETE THIS `DbInfra` struct. THIS WILL BE USED IN EVERY SINGLE REPOSITORY
// FILE YOU CREATE. `DbInfra` IS HOLDING ALL KINDS OF DATABASE INFRASTRUCTURE WHICH YOU CONFUGURED THROUGH
// `env.json`.
type DbInfra struct {
	Conn *bean.DBDeps
}
