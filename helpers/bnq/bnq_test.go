package bnq

import (
	"fmt"
	"log"
	"testing"
)

func TestPublish(t *testing.T) {
	clt := NewRdb("localhost:6381", "secret", 1)
	cmd, err := clt.Publish("ch", Data{
		Key: "aaaa",
		Val: "bbbbb",
	})
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Printf("%+v \n", cmd)
}
func TestInfo(t *testing.T) {

}
