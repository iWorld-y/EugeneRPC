package server

import (
	"EugeneRPC/codec"
	"reflect"
)

type request struct {
	header *codec.Header
	argv   reflect.Value
	replyv reflect.Value
}
