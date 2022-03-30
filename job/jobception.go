package job

import (
	"fmt"
	"sync"
	"time"

	"github.com/gocraft/work"
	redigo "github.com/gomodule/redigo/redis"
	"github.com/spf13/viper"
)

// This is a singleton.
var redisPool *redigo.Pool
var once sync.Once

func Init() *work.Enqueuer {

	once.Do(func() {
		redisPool = InitRedisPool()
	})

	// Make an enqueuer with a particular namespace
	return work.NewEnqueuer(viper.GetString("queue.redis.prefix"), redisPool)
}

func Immediate(jobName string, data map[string]interface{}) error {

	createjob := Init()

	_, err := createjob.Enqueue(jobName, data)

	return err
}

func Latter(jobName string, secondsInTheFuture int64, data map[string]interface{}) error {

	createjob := Init()

	_, err := createjob.EnqueueIn(jobName, secondsInTheFuture, data)

	return err
}

func InitRedisPool() *redigo.Pool {

	password := viper.GetString("queue.redis.password")
	database := viper.GetInt("queue.redis.name")

	connectionString := fmt.Sprintf(
		"%s:%s",
		viper.GetString("queue.redis.host"),
		viper.GetString("queue.redis.port"),
	)

	return &redigo.Pool{
		MaxActive: viper.GetInt("queue.redis.poolsize"),
		MaxIdle:   viper.GetInt("queue.redis.maxidle"),
		Wait:      true,
		Dial: func() (redigo.Conn, error) {
			if password == "" {
				return redigo.Dial("tcp", connectionString, redigo.DialDatabase(database))
			}
			return redigo.Dial("tcp", connectionString, redigo.DialPassword(password), redigo.DialDatabase(database))
		},
		TestOnBorrow: func(c redigo.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}
}
