package server

import (
	"EugeneRPC/codec"
	"reflect"
)

type request struct {
	h            *codec.Header
	argv, replyv reflect.Value
}
