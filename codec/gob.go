package codec

import (
	"bufio"
	"encoding/gob"
	"io"
	"log"
)

type GobCodec struct {
	conn io.ReadWriteCloser // 一个可读可写并且可以关闭的连接
	buf  *bufio.Writer      // 缓存待发送的数据, 可以减少 IO 次数
	dec  *gob.Decoder       // 解码器, 将从 conn 读取到的原始数据解码成 Gob, *反序列化*
	enc  *gob.Encoder       // 编码器, 将 Gob 编码为可发送的 *序列化* 数据
}

// 类型断言, 确保 *GobCodec 类型实现了 Codec 接口
// (*GobCodec)(nil): 这是一个类型转换，它创建了一个 *GobCodec 类型的 nil 指针.
// 由于 nil 指针不指向任何具体的值, 这个表达式实际上是在说:" 即使是一个 nil 的 *GobCodec 也满足 Codec 接口的要求."
var _ Codec = (*GobCodec)(nil)

// ReadHeader 利用 encoding/gob 的 Decode 方法实现读取消息头
func (c *GobCodec) ReadHeader(header *Header) error {
	return c.dec.Decode(header)
}

// ReadBody 利用 encoding/gob 的 Encode 方法实现读取消息体
func (c *GobCodec) ReadBody(body interface{}) error {
	return c.dec.Decode(body)
}

// Write 将 RPC 调用的消息头和消息体编码后写入到底层的连接中
func (c *GobCodec) Write(header *Header, body interface{}) (err error) {
	defer func() {
		// 刷新缓冲区
		_ = c.buf.Flush()
		if err != nil {
			// 若有错误则关闭连接
			_ = c.Close()
		}
	}()

	// 编码 header
	if err := c.enc.Encode(header); err != nil {
		log.Println("rpc codec: gob error encoding header:\t", err)
		return err
	}

	// 编码 body
	if err := c.enc.Encode(body); err != nil {
		log.Println("rpc codec: gob error encoding body:\t", err)
		return err
	}
	return nil
}
func (c *GobCodec) Close() error {
	return c.conn.Close()
}

// NewGobCodec 类似工厂模式, 创建并返回一个 GobCodec 实例.
func NewGobCodec(conn io.ReadWriteCloser) Codec {
	buf := bufio.NewWriter(conn)
	return &GobCodec{
		conn: conn,
		buf:  buf,
		dec:  gob.NewDecoder(conn),
		enc:  gob.NewEncoder(buf),
	}
}
