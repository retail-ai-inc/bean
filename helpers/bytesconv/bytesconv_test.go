package bytesconv

import (
	"fmt"
	"testing"
)

func TestBytesToString(t *testing.T) {
	bt := []byte("aa")
	fmt.Println(BytesToString(bt))
}

func TestStringToBytes(t *testing.T) {
	str := "aaa"
	fmt.Println(StringToBytes(str))
}
