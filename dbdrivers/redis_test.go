package dbdrivers

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

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
				host:               "34.84.56.20,35.200.65.32,35.243.97.67,35.243.119.218,35.187.198.109,104.198.85.119",
				port:               "6379",
				dbName:             0,
				maxretries:         -2,
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

func TestRand(t *testing.T) {
	var wg sync.WaitGroup
	var count = 10000
	wg.Add(count)
	for i := 0; i < count; i++ {
		go func() {
			fmt.Printf("%d,", rand.Intn(count))
			wg.Done()
		}()
	}
	wg.Wait()
}
