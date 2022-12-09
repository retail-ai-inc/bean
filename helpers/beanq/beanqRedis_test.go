package beanq

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/retail-ai-inc/bean/helpers/beanq/json"
	"github.com/spf13/cast"
	"log"
	"testing"
	"time"
)

var (
	options = Options{
		RedisOptions: &redis.Options{
			Addr:      "localhost:6381",
			Dialer:    nil,
			OnConnect: nil,
			Username:  "",
			Password:  "secret",
			DB:        2,
		},
	}
	queue    = "ch2"
	group    = "g2"
	consumer = "cs1"
	clt      Beanq
)

func init() {
	clt = NewBeanq("redis", options)
}

/*
  - TestPublish
  - @Description:
    publisher
  - @param t
*/
func TestPublish1(t *testing.T) {

	for i := 0; i < 5; i++ {
		m := make(map[int]string)
		m[i] = "k----" + cast.ToString(i)

		d, _ := json.Marshal(m)
		task := NewTask("", d)
		cmd, err := clt.Publish(task, Queue("ch2"))
		if err != nil {
			log.Fatalln(err)
		}
		fmt.Printf("%+v \n", cmd)
	}
	defer clt.Close()
}
func TestDelayPublish(t *testing.T) {
	m := make(map[string]string)
	m["delayMsg"] = "new msg11111"
	b, _ := json.Marshal(&m)
	task := NewTask("update", b)

	delayT := time.Now().Add(100 * time.Second)
	_, err := clt.DelayPublish(task, delayT, Queue("delay-ch"))
	if err != nil {
		t.Fatal(err.Error())
	}
	defer clt.Close()
}

func TestXInfo(t *testing.T) {
	ctx := context.Background()

	clt := NewRedis(options)

	cmd := clt.client.XInfoStream(ctx, queue)
	fmt.Printf("%+v \n", cmd.Val())
	groupCmd := clt.client.XInfoGroups(ctx, queue)
	fmt.Printf("%+v \n", groupCmd.Val())
}
func TestPending(t *testing.T) {
	ctx := context.Background()
	clt := NewRedis(options)

	cmd := clt.client.XPending(ctx, queue, group)
	fmt.Printf("%+v \n", cmd.Val())
}
func TestInfo(t *testing.T) {
	ctx := context.Background()
	clt := NewRedis(options)

	cmd := clt.client.Info(ctx)

	fmt.Printf("%+v \n", cmd.Val())
}
func TestMemoryUsage(t *testing.T) {
	ctx := context.Background()
	clt := NewRedis(options)
	cmd := clt.client.MemoryUsage(ctx, "success")
	fmt.Printf("%+v \n", cmd)
}
func TestClaim(t *testing.T) {
	clt := NewRedis(options)
	qu := "c11"

	t.Run("publish", func(t *testing.T) {
		m := make(map[string]string)
		m["delayMsg"] = "delayMsg"
		d, _ := json.Marshal(m)
		task := NewTask("", d)
		//dua, _ := time.ParseDuration("50s")
		//ext := time.Now().Add(dua).Unix()
		r, err := clt.Publish(task, Queue(qu))
		if err != nil {
			t.Fatal(err.Error())
		}
		t.Fatalf("发布消息：%+v \n", r)
	})

	t.Run("creategroup", func(t *testing.T) {
		if err := clt.createGroup(qu, "g11"); err != nil {
			t.Fatal(err.Error())
		}
	})

	t.Run("claim", func(t *testing.T) {

		//streams, err := clt.client.XReadGroup(clt.ctx, &redis.XReadGroupArgs{
		//	Group:    "g11",
		//	Consumer: "",
		//	Streams:  []string{qu, ">"},
		//	Count:    0,
		//	Block:    0,
		//	NoAck:    false,
		//}).Result()
		//fmt.Printf("streams:%+v \n", streams)
		datas, err := clt.client.XPendingExt(clt.ctx, &redis.XPendingExtArgs{
			Stream: qu,
			Group:  "g11",
			Idle:   0,
			Start:  "-",
			End:    "+",
			Count:  10,
			//Consumer: "aa",
		}).Result()
		if err != nil {
			t.Fatal(err.Error())
		}
		fmt.Printf("Claims:%+v \n", datas)

		for _, v := range datas {
			fmt.Println(v.Idle.Seconds())
			fmt.Println(time.Now().Second())
			err := clt.client.XClaim(clt.ctx, &redis.XClaimArgs{
				Stream:   qu,
				Group:    "g11",
				Consumer: v.Consumer,
				MinIdle:  0,
				Messages: []string{v.ID},
			}).Err()
			if err != nil {
				t.Fatal(err.Error())
			}
			clt.client.XAck(clt.ctx, qu, "g11", v.ID)
			fmt.Printf("ID:%s \n", v.ID)
			break
		}

	})
}
func TestRetry(t *testing.T) {
	retry(func() bool {
		fmt.Println("aa")
		return false
	}, 3*time.Second)
}

var retryFlag chan bool = make(chan bool)

func retry(f func() bool, delayTime time.Duration) {
	index := 1

	//ticker := time.NewTicker(delayTime)
	//defer ticker.Stop()
	//for{
	//	select {
	//	case <-ticker.C:
	//
	//	}
	//}
	for {
		go time.AfterFunc(delayTime, func() {
			retryFlag <- f()
		})
		if <-retryFlag {
			return
		}
		if index == 3 {
			return
		}
		index++
	}
}
