package server

import (
	"EugeneRPC/codec"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"reflect"
	"sync"
)

var DefaultServer *Server

type Server struct{}

func NewServer() *Server {
	DefaultServer = &Server{}
	return DefaultServer
}

// Accept 循环等待 socket 建立连接, 若连接无错误则开启 server.serveConn 子协程去处理具体过程.
func (server *Server) Accept(listener net.Listener) {
	for {
		// 若无连接则阻塞
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

var invalidRequest = struct{}{}

func (server *Server) serveCodec(cc codec.Codec) {
	sending := new(sync.Mutex)
	waitGroup := new(sync.WaitGroup)
	for {
		req, err := server.readRequest(cc)
		if err != nil {
			if req == nil {
				break
			}
			req.header.Error = err.Error()
			server.sendResponse(cc, req.header, invalidRequest, sending)
			continue
		}
		waitGroup.Add(1)
		go server.handleRequest(cc, req, sending, waitGroup)
	}
}

func (server *Server) readRequestHeader(cc codec.Codec) (*codec.Header, error) {
	var header codec.Header
	// 传入的结构体 cc 要求实现了 codec.Codec 的所有方法
	// 所以可以直接调用其已实现的 ReadHeader 方法
	if err := cc.ReadHeader(&header); err != nil {
		if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
			log.Println("RPC 服务: header 读取错误: ", err)
		}
		return nil, err
	}
	return &header, nil
}

// readRequest 读取请求
func (server *Server) readRequest(cc codec.Codec) (*request, error) {
	// 先解析请求头
	header, err := server.readRequestHeader(cc)
	if err != nil {
		return nil, err
	}
	req := &request{header: header}

	// 再利用反射获取请求参数
	// TODO 此时不知道请求参数的具体类型, 当前版本先假设其为字符串类型
	// reflect.TypeOf("") 获取传入参数的类型
	// reflect.New(t) 分配内存并返回一个指向新分配零值的 t 对象
	// 创建了一个类型为string的空接口值
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
	log.Println(req.header, req.argv.Elem())
	req.replyv = reflect.ValueOf(fmt.Sprintf("EugeneRPC resp %d", req.header.SequenceNum))
	server.sendResponse(cc, req.header, req.replyv.Interface(), sending)
}

func Accept(listener net.Listener) {
	DefaultServer.Accept(listener)
}
