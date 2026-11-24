package dbdrivers

// Logger is the minimal leveled logger used by the database drivers.
// It is satisfied by bean's `log.Logger` (and `*github.com/labstack/gommon/log.Logger`).
type Logger interface {
	Debugf(format string, args ...interface{})
	Warnf(format string, args ...interface{})
}
