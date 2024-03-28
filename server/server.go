package server

import (
	codec "EugeneRPC/codec"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"reflect"
	"sync"
)

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

type Server struct {
}

func NewServer() *Server {
	return &Server{}
}

var DefaultServer = NewServer()

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
			req.header.Error = err.Error()
			server.sendResponse(cc, req.header, inalidRequest, sending)
			continue
		}
		waitGroup.Add(1)
		go server.handleRequest(cc, req, sending, waitGroup)
	}
}

type request struct {
	header *codec.Header
	argv   reflect.Value
	replyv reflect.Value
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
