package server

import "EugeneRPC/codec"

// MagicNumber 用于标识这是一个 EugeneRPC 的请求 (类似一种水印)
const MagicNumber = 0x1234abcd

type Option struct {
	MagicNumber int        // 水印标识
	CodecType   codec.Type // 客户端选择的编解码格式
}

var DefaultOption = &Option{
	MagicNumber: MagicNumber,
	CodecType:   codec.GobType,
}
