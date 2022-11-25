package beanq

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/retail-ai-inc/bean/helpers/beanq/stringx"
	"sync"
	"time"
)

var (
	once   sync.Once
	client *redis.Client
)

type DoConsumer func(*Task, *redis.Client) error

type Rdb struct {
	client *redis.Client
	ctx    context.Context

	broker                   string
	keepJobInQueue           time.Duration
	keepFailedJobsInHistory  time.Duration
	keepSuccessJobsInHistory time.Duration

	jobMaxRetry int
	prefix      string
	count       int64
}

func NewRdb(count int64, options *redis.Options) *Rdb {
	ctx := context.Background()
	once.Do(func() {
		client = redis.NewClient(options)
	})
	return &Rdb{client: client, ctx: ctx, count: count}
}
func (t *Rdb) Publish(task *Task, option ...Option) (*redis.StringCmd, error) {

	opt, err := composeOptions(option...)
	if err != nil {
		return nil, err
	}
	strcmd := t.client.XAdd(t.ctx, &redis.XAddArgs{
		Stream: opt.queue,
		Limit:  0,
		MaxLen: 0,
		Values: map[string]any{"payload": task.payload},
	})
	if strcmd.Err() != nil {
		return nil, strcmd.Err()
	}
	if err := t.createGroup(opt.queue, opt.group); err != nil {
		return nil, err
	}
	return strcmd, nil

}

func (t *Rdb) Consumer(group, queue string, f DoConsumer) error {

	consumerUuid, err := uuid.NewUUID()
	if err != nil {
		return err
	}

	ch := make(chan redis.XStream)

	go func() {
		for {
			t.client.XInfoGroups(t.ctx, "")

			cmd := t.client.XReadGroup(t.ctx, &redis.XReadGroupArgs{
				Group:    group,
				Streams:  []string{queue, ">"},
				Consumer: consumerUuid.String(),
				Count:    t.count,
				Block:    0,
			})
			err = cmd.Err()
			if err != nil {
				fmt.Printf("CMD Error:%+v \n", err)
				continue
			}
			if len(cmd.Val()) <= 0 {
				continue
			}
			for _, v := range cmd.Val() {
				ch <- v
			}
		}
	}()
	for v := range ch {
		task := &Task{
			name: v.Stream,
		}
		for _, vm := range v.Messages {
			task.id = vm.ID
			if payload, ok := vm.Values["payload"]; ok {
				task.payload = stringx.StringToByte(payload.(string))
			}
			now := time.Now()
			err := f(task, t.client)
			if err != nil {
				continue
			}
			sub := time.Now().Sub(now)
			fmt.Printf("ExecutionTime:%+v,Consumer:%s \n", sub, task.id)
		}
	}
	return nil
}

/*
  - createGroup
  - @Description:
    if group not exist,then create it
  - @receiver t
  - @param queue
  - @param group
  - @return error
*/
func (t *Rdb) createGroup(queue, group string) error {
	groupCmd := t.client.XInfoGroups(t.ctx, queue)
	if groupCmd.Err() != nil {
		return groupCmd.Err()
	}
	var b bool
	vals := groupCmd.Val()
	for _, val := range vals {
		if val.Name == group {
			b = true
			break
		}
	}
	if b {
		return nil
	}

	if err := t.client.XGroupCreate(t.ctx, queue, group, "0").Err(); err != nil {
		return err
	}

	return nil
}

/*
  - Close
  - @Description:
    close redis client
  - @receiver t
  - @return error
*/
func (t *Rdb) Close() error {
	return t.client.Close()
}
