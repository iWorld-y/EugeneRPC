package server

import (
	codec "EugeneRPC/codec"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"reflect"
	"sync"
)

type Server struct {
}

func NewServer() *Server {
	return &Server{}
}

var DefaultServer = NewServer()

// Accept 循环等待 socket 建立连接, 若连接无错误则开启 server.serveConn 子协程去处理具体过程.
func (server *Server) Accept(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("RPC 服务: Accept 错误: ", err)
			return
		}
		go server.ServeConn(conn)
	}
}

// ServeConn 对于 Option 的解析默认使用 JSON 格式.
// 若 MagicNumber 和 编解码器类型 都正确,
// 则将调用对应的编解码器处理 conn 后得到的 Codec 实例传入 server.serveCodec() 中
func (server *Server) ServeConn(conn io.ReadWriteCloser) {
	defer func() { _ = conn.Close() }()
	var opt Option
	if err := json.NewDecoder(conn).Decode(&opt); err != nil {
		log.Println("RPC 服务: options 错误: ", err)
		return
	}
	if opt.MagicNumber != MagicNumber {
		log.Printf("RPC 服务: 水印数字错误: %x", opt.MagicNumber)
		return
	}
	f := codec.NewCodecFuncMap[opt.CodecType]
	if f == nil {
		log.Printf("RPC 服务: 编解码器类型错误: %s", opt.CodecType)
		return
	}
	// f(conn) 会返回一个 GobCodec 或其它类型的结构体实例
	server.serveCodec(f(conn))
}

var inalidRequest = struct{}{}

func (server *Server) serveCodec(cc codec.Codec) {
	sending := new(sync.Mutex)
	waitGroup := new(sync.WaitGroup)
	for {
		req, err := server.readRequest(cc)
		if err != nil {
			if req == nil {
				break
			}
			req.h.Error = err.Error()
			server.sendResponse(cc, req.h, inalidRequest, sending)
			continue
		}
		waitGroup.Add(1)
		go server.handleRequest(cc, req, sending, waitGroup)
	}
}

func (server *Server) readRequestHeader(cc codec.Codec) (*codec.Header, error) {
	var h codec.Header
	if err := cc.ReadHeader(&h); err != nil {
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			log.Println("RPC 服务: header 读取错误: ", err)
		}
		return nil, err
	}
	return &h, nil
}
func (server *Server) readRequest(cc codec.Codec) (*request, error) {
	h, err := server.readRequestHeader(cc)
	if err != nil {
		return nil, err
	}
	req := &request{h: h}

	req.argv = reflect.New(reflect.TypeOf(""))
	if err = cc.ReadBody(req.argv.Interface()); err != nil {
		log.Println("RPC 服务: argv 读取错误: ", err)
	}
	return req, nil
}

func (server *Server) sendResponse(cc codec.Codec, header *codec.Header, body interface{}, sending *sync.Mutex) {
	sending.Lock()
	defer sending.Unlock()
	if err := cc.Write(header, body); err != nil {
		log.Println("RPC 服务: response 写入错误: ", err)
	}
}

func (server *Server) handleRequest(cc codec.Codec, req *request, sending *sync.Mutex, waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()
	log.Println(req.h, req.argv.Elem())
	req.replyv = reflect.ValueOf(fmt.Sprintf("EugeneRPC resp %d", req.h.SequenceNum))
	server.sendResponse(cc, req.h, req.replyv.Interface(), sending)
}

func Accept(listener net.Listener) {
	DefaultServer.Accept(listener)
}
