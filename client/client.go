package client

import (
	"EugeneRPC/codec"
	"EugeneRPC/server"
	"errors"
	"fmt"
	"io"
	"sync"
)

type Call struct {
	Seq           uint64
	ServiceMethod string
	Args          interface{}
	Reply         interface{}
	Error         error
	Done          chan *Call
}

func (c *Call) done() {
	c.Done <- c
}

type Client struct {
	cc       codec.Codec
	opt      *server.Option
	seeding  sync.Mutex
	header   codec.Header
	mu       sync.Mutex
	seq      uint64
	pending  map[uint64]*Call
	closing  bool
	shutdown bool
}

var (
	// 将 nil 指针强行转为 *Client 类型, 并赋值给 io.Closer 类型的 _ 变量
	// 若 Client 未实现 io.Closer 接口则无法通过编译检查
	_           io.Closer = (*Client)(nil)
	ErrShutdown           = errors.New("连接已关闭")
)

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closing {
		return ErrShutdown
	}
	c.closing = true
	return c.cc.Close()
}

func (c *Client) IsAvailable() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.shutdown && !c.closing
}

func (c *Client) registerCall(call *Call) (uint64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// 无服务
	if c.closing || c.shutdown {
		return 0, ErrShutdown
	}
	call.Seq = c.seq
	c.seq++
	c.pending[call.Seq] = call
	return call.Seq, nil
}

func (c *Client) removeCall(seq uint64) (*Call, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if call, ok := c.pending[seq]; ok {
		delete(c.pending, seq)
		return call, nil
	}
	return nil, ErrShutdown
}

func (c *Client) terminateCalls(err error) {
	// 锁住 seeding, 保证在这段代码执行期间, 其他任何试图访问 seeding 的协程都被阻塞
	c.seeding.Lock()
	defer c.seeding.Unlock()
	// 锁住 mu 互斥锁
	c.mu.Lock()
	defer c.mu.Unlock()
	c.shutdown = true
	for _, call := range c.pending {
		call.Error = err
		call.done()
	}
}

func (c *Client) receive() {
	var err error
	for err == nil {
		h := &codec.Header{}
		if err = c.cc.ReadHeader(h); err != nil {
			break
		}
		call := &Call{}
		call, _ = c.removeCall(h.SequenceNum)
		switch {
		// call 不存在
		case call == nil:
			err = c.cc.ReadBody(nil)
		// call 存在, 但服务端处理有错误
		case h.Error != "":
			call.Error = fmt.Errorf(h.Error)
			err = c.cc.ReadBody(nil)
			call.done()
		// 一切正常
		default:
			err = c.cc.ReadBody(call.Reply)
			if err != nil {
				call.Error = errors.New("读取消息体失败: \t" + err.Error())
			}
			call.done()
		}
	}
	c.terminateCalls(err)
}
