package dbdrivers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// this is for redis v6.x
func Test_connectRedisDB(t *testing.T) {
	type args struct {
		password           string
		host               string
		port               string
		dbName             int
		maxretries         int
		poolsize           int
		minIdleConnections int
		dialTimeout        time.Duration
		readTimeout        time.Duration
		writeTimeout       time.Duration
		poolTimeout        time.Duration
		readOnly           bool
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "success",
			args: args{
				password:           "AqNUe43qWL",
				host:               "34.84.252.114:6379,34.85.78.190:6379,34.146.251.57:6379,34.146.231.90:6379,34.146.240.130:6379,35.243.103.175:6379",
				port:               "6379",
				dbName:             0,
				maxretries:         0,
				poolsize:           0,
				minIdleConnections: 0,
				dialTimeout:        0,
				readTimeout:        0,
				writeTimeout:       0,
				poolTimeout:        0,
				readOnly:           false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				connectRedisDB(tt.args.password, tt.args.host, tt.args.port, tt.args.dbName, tt.args.maxretries, tt.args.poolsize, tt.args.minIdleConnections, tt.args.dialTimeout, tt.args.readTimeout, tt.args.writeTimeout, tt.args.poolTimeout, tt.args.readOnly)
			})
		})
	}
}
