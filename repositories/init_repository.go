/*
 * Copyright The RAI Inc.
 * The RAI Authors
 *
 * *** PLEASE DO NOT DELETE THIS FILE. ***
 */

package repositories

import (
	"bean/internals/global"
)

/*
 * XXX: IMPORTANT -  DO NOT DELETE THIS `DbInfra` struct. THIS WILL BE USED IN EVERY SINGLE REPOSITORY
 * FILE YOU CREATE. `DbInfra` IS HOLDING ALL KINDS OF DATABASE INFRASTRUCTURE WHICH YOU CONFUGURED THROUGH
 * `env.json`.
 */
type DbInfra struct {
	Conn *global.DBDeps
}
