package beanq

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/retail-ai-inc/bean/helpers/beanq/json"
	"github.com/retail-ai-inc/bean/helpers/beanq/stringx"
	"github.com/retail-ai-inc/bean/helpers/beanq/timex"
	"github.com/spf13/cast"
	"runtime/debug"
	"sync"
	"time"
)

var (
	once   sync.Once
	client *redis.Client
)

type DoConsumer func(*Task, *redis.Client) error

type BeanqRedis struct {
	client *redis.Client
	ctx    context.Context
	wg     *sync.WaitGroup
	ch     chan redis.XStream
	stop   chan struct{} // goroutines stop
	done   chan struct{} // task has done
	err    chan error

	broker                   string
	keepJobInQueue           time.Duration
	keepFailedJobsInHistory  time.Duration
	keepSuccessJobsInHistory time.Duration

	minWorkers  int
	jobMaxRetry int
	prefix      string
}

func NewRedis(options Options) *BeanqRedis {
	ctx := context.Background()
	once.Do(func() {
		client = redis.NewClient(options.RedisOptions)
	})
	return &BeanqRedis{
		client:                   client,
		ctx:                      ctx,
		wg:                       &sync.WaitGroup{},
		ch:                       make(chan redis.XStream),
		stop:                     make(chan struct{}),
		done:                     make(chan struct{}),
		err:                      make(chan error),
		minWorkers:               options.MinWorkers,
		jobMaxRetry:              options.JobMaxRetry,
		prefix:                   options.Prefix,
		keepJobInQueue:           options.KeepJobInQueue,
		keepFailedJobsInHistory:  options.KeepFailedJobsInHistory,
		keepSuccessJobsInHistory: options.KeepSuccessJobsInHistory,
	}
}
func (t *BeanqRedis) DelayPublish(task *Task, delayTime time.Time, option ...Option) (*Result, error) {
	option = append(option, ExecuteTime(delayTime))
	opt, err := composeOptions(option...)
	if err != nil {
		return nil, err
	}
	task.executeTime = delayTime
	task.addTime = time.Now().Format(timex.DateTime)

	//format data
	msg := make(map[string]any)
	msg["payload"] = stringx.ByteToString(task.payload)
	msg["name"] = task.Name()
	msg["executeTime"] = delayTime
	msg["addTime"] = task.addTime
	data, err := json.Json.MarshalToString(msg)

	if err != nil {
		return nil, err
	}
	//fmt.Println(data)
	if err := t.client.LPush(t.ctx, opt.queue, data).Err(); err != nil {
		return nil, err
	}
	return nil, nil
}
func (t *BeanqRedis) Publish(task *Task, option ...Option) (*Result, error) {

	opt, err := composeOptions(option...)
	if err != nil {
		return nil, err
	}
	id := task.name
	if id == "" {
		id = "*"
	}
	values := make(map[string]any)
	values["payload"] = task.payload
	values["addtime"] = time.Now().Format(timex.DateTime)

	if opt.executeTime != 0 {
		values["executeTime"] = opt.executeTime
	}
	fmt.Printf("Values:%+v \n", values)
	strcmd := t.client.XAdd(t.ctx, &redis.XAddArgs{
		Stream:     opt.queue,
		NoMkStream: false,
		MaxLen:     opt.maxLen,
		MinID:      "",
		Approx:     false,
		//Limit:      0,
		ID:     id,
		Values: values,
	})
	if err := strcmd.Err(); err != nil {
		return nil, err
	}

	return &Result{Args: strcmd.Args(), Id: strcmd.Val()}, nil
}
func (t *BeanqRedis) Run(server *Server) {

	consumers := server.consumers()
	workers := make(chan struct{}, t.minWorkers)

	for _, v := range consumers {

		err := t.createGroup(v.queue, v.group)
		if err != nil {
			fmt.Printf("CreateGroupErr:%+v \n", err)
			continue
		}

		workers <- struct{}{}
		go t.work(v, server, workers)
	}
	//https://redis.io/commands/xclaim/
	//monitor other stream pending
	go t.claim(consumers)
	go t.delayConsumer()
	//catch errors
	err := t.getErrors()
	fmt.Printf("Err:%s \n", err.Error())

	<-t.done
}

/*
* work
*  @Description:
*  @receiver t
* @param handler
* @param server
* @param workers
 */
func (t *BeanqRedis) work(handler *consumerHandler, server *Server, workers chan struct{}) {
	defer close(t.done)
	if err := t.readGroups(handler.queue, handler.group, server.count); err != nil {
		t.err <- err
		return
	}
	t.consumerMsgs(handler.consumerFun, handler.group)
	<-workers
}

/*
  - delayConsumer
  - @Description:
    now testing,need to optimize
  - @receiver t
*/
func (t *BeanqRedis) delayConsumer() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	jn := json.Json

	for {
		select {
		case <-t.stop:
			return
		case <-ticker.C:
			result, err := t.client.LRange(t.ctx, "delay-ch", 0, 10).Result()
			if err != nil {
				fmt.Printf("LRangeError:%s \n", err.Error())
				t.err <- fmt.Errorf("LRangeErr:%s,Stack:%v", err.Error(), stringx.ByteToString(debug.Stack()))
				continue
			}
			//those codes need to improve
			taskV := struct {
				Id          string    `json:"id"`
				Name        string    `json:"name"`
				PayLoad     string    `json:"payLoad"`
				AddTime     string    `json:"addTime"`
				ExecuteTime time.Time `json:"executeTime"`
			}{}

			for _, s := range result {
				if err := jn.Unmarshal([]byte(s), &taskV); err != nil {
					fmt.Printf("UnmarshalError:%s \n", err.Error())
					t.err <- fmt.Errorf("UnmarshalErr:%s,Stack:%v", err.Error(), stringx.ByteToString(debug.Stack()))
					continue
				}
				task := Task{
					payload:     stringx.StringToByte(taskV.PayLoad),
					addTime:     taskV.AddTime,
					executeTime: taskV.ExecuteTime,
				}

				if taskV.ExecuteTime.Before(time.Now()) {
					_, err := t.Publish(&task, Queue(defaultOptions.defaultDelayQueueName))
					if err != nil {
						fmt.Printf("PublishError:%s \n", err.Error())
						t.err <- fmt.Errorf("PublishErr:%s,Stack:%v", err.Error(), stringx.ByteToString(debug.Stack()))
						continue
					}
					if err := t.client.LRem(t.ctx, "delay-ch", 1, s).Err(); err != nil {
						fmt.Printf("LRemErr:%s \n", err.Error())
						t.err <- fmt.Errorf("LRemErr:%s,Stack:%v", err.Error(), stringx.ByteToString(debug.Stack()))
					}
					continue
				}

				if err := t.client.LRem(t.ctx, "delay-ch", 1, s).Err(); err != nil {
					fmt.Printf("LRemErr:%s \n", err.Error())
					t.err <- fmt.Errorf("LRemErr:%s,Stack:%v", err.Error(), stringx.ByteToString(debug.Stack()))
					continue
				}
				if err := t.client.RPush(t.ctx, "delay-ch", s).Err(); err != nil {
					fmt.Printf("RPushError:%s \n", err.Error())
					t.err <- fmt.Errorf("RPushErr:%s,Stack:%v", err.Error(), stringx.ByteToString(debug.Stack()))
					continue
				}
			}
		}
	}
}

/*
  - claim
  - @Description:
    need test
    this function can't work,developing
  - @receiver t
*/
func (t *BeanqRedis) claim(consumers []*consumerHandler) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-t.stop:
			return
		case <-ticker.C:
			start := "-"
			end := "+"

			for _, consumer := range consumers {

				res, err := t.client.XPendingExt(t.ctx, &redis.XPendingExtArgs{
					Stream: consumer.queue,
					Group:  consumer.group,
					Start:  start,
					End:    end,
					//Count:  10,
				}).Result()
				if err != nil && err != redis.Nil {
					t.err <- fmt.Errorf("XPendingErr:%s,Stack:%v", err.Error(), stringx.ByteToString(debug.Stack()))
					break
				}
				for _, v := range res {

					if v.Idle.Seconds() >= 10 {

						claims, err := t.client.XClaim(t.ctx, &redis.XClaimArgs{
							Stream:   consumer.queue,
							Group:    consumer.group,
							Consumer: consumer.queue,
							MinIdle:  10 * time.Second,
							Messages: []string{v.ID},
						}).Result()
						if err != nil && err != redis.Nil {
							t.err <- fmt.Errorf("XClaimErr:%s,Stack:%v", err.Error(), stringx.ByteToString(debug.Stack()))
							continue
						}
						if err := t.client.XAck(t.ctx, consumer.queue, consumer.group, v.ID).Err(); err != nil {
							t.err <- fmt.Errorf("XAckErr:%s,Stack:%v", err.Error(), stringx.ByteToString(debug.Stack()))
							continue
						}
						fmt.Printf("claim:%+v \n", claims)
						t.ch <- redis.XStream{
							Stream:   consumer.queue,
							Messages: claims,
						}
					}

				}

			}
		}
	}
}
func (t *BeanqRedis) readGroups(queue, group string, count int64) error {

	go func() {

		for {
			select {
			case <-t.stop:
				return
			default:
				streams, err := t.client.XReadGroup(t.ctx, &redis.XReadGroupArgs{
					Group:    group,
					Streams:  []string{queue, ">"},
					Consumer: queue,
					Count:    count,
					Block:    0,
				}).Result()

				if err != nil {
					t.err <- fmt.Errorf("XReadGroupErr:%s,Stack:%v", err.Error(), stringx.ByteToString(debug.Stack()))
					continue
				}
				if len(streams) <= 0 {
					continue
				}

				for _, v := range streams {
					t.ch <- v
				}
			}
		}
	}()
	return nil
}
func (t *BeanqRedis) consumerMsgs(f DoConsumer, group string) {
	info := SuccessInfo
	result := &ConsumerResult{
		Level:   InfoLevel,
		Info:    info,
		RunTime: "",
	}
	var now time.Time

	for {
		select {
		case <-t.stop:
			return
		case msg := <-t.ch:
			task := &Task{
				name: msg.Stream,
			}
			for _, vm := range msg.Messages {

				task.id = vm.ID
				if payload, ok := vm.Values["payload"]; ok {
					task.payload = stringx.StringToByte(payload.(string))
				}
				if addtime, ok := vm.Values["addtime"]; ok {
					task.addTime = addtime.(string)
				}
				//fmt.Printf("task1:%+v \n", msg)
				now = time.Now()
				if executeT, ok := vm.Values["executeTime"]; ok {
					if cast.ToInt64(executeT) > now.Unix() {
						continue
					}
				}
				//fmt.Printf("task2:%+v \n", msg)
				err := f(task, t.client)
				if err != nil {
					info = FailedInfo
					result.Level = ErrLevel
					result.Info = flagInfo(err.Error())
				}

				sub := time.Now().Sub(now)

				result.Payload = task.payload
				result.AddTime = time.Now().Format(timex.DateTime)
				result.RunTime = sub.String()
				result.Queue = msg.Stream
				result.Group = group

				b, err := json.Marshal(result)
				if err != nil {
					t.err <- fmt.Errorf("JsonMarshalErr:%s,Stack:%v", err.Error(), stringx.ByteToString(debug.Stack()))
					continue
				}

				//ack
				if err := t.client.XAck(t.ctx, msg.Stream, group, vm.ID).Err(); err != nil {
					fmt.Printf("ACK Error:%s \n", err.Error())
					continue
				}

				if err := t.client.LPush(t.ctx, string(info), b).Err(); err != nil {
					t.err <- fmt.Errorf("LPushErr:%s,Stack:%v", err.Error(), stringx.ByteToString(debug.Stack()))
					continue
				}
			}
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
func (t *BeanqRedis) createGroup(queue, group string) error {

	err := t.client.XGroupCreateMkStream(t.ctx, queue, group, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return err
	}
	return nil

}
func (t *BeanqRedis) getErrors() (err error) {
	for {
		select {
		case errc := <-t.err:
			err = errc
			break
		}
	}
	return
}

/*
  - Close
  - @Description:
    close redis client
  - @receiver t
  - @return error
*/
func (t *BeanqRedis) Close() error {
	close(t.stop)
	close(t.ch)
	return t.client.Close()
}
