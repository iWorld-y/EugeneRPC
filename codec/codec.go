package codec

import "io"

type Header struct {
	ServiceMethod string // 指定服务与方法
	SequenceNum   uint64 // 客户端指定的序列号, 也可理解为某个请求的 ID, 可以区分不同的请求
	Error         string // 错误信息, 客户端置空, 服务端若有错误才填写
}

// Codec 对消息体进行编解码 "coder & decoder"
type Codec interface {
	io.Closer                         // 需实现关闭方法
	ReadHeader(*Header) error         // 编解码消息头
	ReadBody(interface{}) error       // 编解码消息体
	Write(*Header, interface{}) error // 将消息头和消息体组装为消息
}

// NewCodecFunc 是一个函数类型的别名,
// 该类型的函数接收 io.ReadWriteCloser 类型参数,
// 并返回 实现了 Codec 的实例
type NewCodecFunc func(closer io.ReadWriteCloser) Codec

type Type string

// NewCodecFuncMap 是 类型 ==> 函数 的映射, 用于 *注册* 和 *查找* 编解码器 (codec)
var NewCodecFuncMap map[Type]NewCodecFunc

// 定义不同的编解码器
const (
	GobType  Type = "application/gob" // Gob: Go Binary
	JsonType Type = "application/json"
)

func init() {
	NewCodecFuncMap = make(map[Type]NewCodecFunc)
	// 注册 GobCodec 编解码器的创建函数
	NewCodecFuncMap[GobType] = NewGobCodec
}
