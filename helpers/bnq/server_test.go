package bnq

import (
	"fmt"
	"testing"
)

func TestConsumer(t *testing.T) {
	group := "ch-group"
	queue := "ch"

	rdb := NewRdb("localhost:6381", "secret", 1)
	datas := rdb.Consumer(group, queue, "cs1")

	for v := range datas {
		fmt.Printf("%+v \n", v)
	}
}
