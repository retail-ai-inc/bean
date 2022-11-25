package stringx

import "unsafe"

func StringToByte(str string) []byte {
	return *(*[]byte)(unsafe.Pointer(&str))
}
func ByteToString(data []byte) string {
	return *(*string)(unsafe.Pointer(&data))
}
