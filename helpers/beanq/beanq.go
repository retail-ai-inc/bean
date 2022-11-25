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

type Beanq struct {
	client *redis.Client
	ctx    context.Context

	broker                   string
	keepJobInQueue           time.Duration
	keepFailedJobsInHistory  time.Duration
	keepSuccessJobsInHistory time.Duration

	minWorkers  int
	jobMaxRetry int
	prefix      string
	count       int64
}

func NewBeanq(count int64, options *redis.Options) *Beanq {
	ctx := context.Background()
	once.Do(func() {
		client = redis.NewClient(options)
	})
	return &Beanq{client: client, ctx: ctx, count: count}
}
func (t *Beanq) Publish(task *Task, option ...Option) (*redis.StringCmd, error) {

	opt, err := composeOptions(option...)
	if err != nil {
		return nil, err
	}
	id := task.name
	if id == "" {
		id = "*"
	}
	strcmd := t.client.XAdd(t.ctx, &redis.XAddArgs{
		Stream: opt.queue,
		Limit:  0,
		MaxLen: 0,
		ID:     id,
		Values: map[string]any{"payload": task.payload},
	})
	if err := strcmd.Err(); err != nil {
		return nil, err
	}
	if err := t.createGroup(opt.queue, opt.group); err != nil {
		return nil, err
	}
	return strcmd, nil

}
func (t *Beanq) Run(server *Server) {
	consumers := server.consumers()

	//need to control the number of goroutine
	//will optimize
	for _, v := range consumers {
		go func() {
			ch, _ := t.readGroups(v.queue, v.group)
			t.consumerMsgs(ch, v.consumer)
		}()
	}
	select {}
}
func (t *Beanq) readGroups(queue, group string) (<-chan redis.XStream, error) {
	consumerUuid, err := uuid.NewUUID()
	if err != nil {
		return nil, err
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
	return ch, nil
}
func (t *Beanq) consumerMsgs(ch <-chan redis.XStream, f DoConsumer) {
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
				//failed jobs
				//....to do
				continue
			}
			sub := time.Now().Sub(now)
			fmt.Printf("ExecutionTime:%+v,Consumer:%s \n", sub, task.id)
		}
	}
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
func (t *Beanq) createGroup(queue, group string) error {
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
func (t *Beanq) Close() error {
	return t.client.Close()
}
