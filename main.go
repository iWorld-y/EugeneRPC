package main

import (
	"EugeneRPC/codec"
	"EugeneRPC/server"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"time"
)

func startServer(addr chan string) {
	// pick a free port
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatal("network error:", err)
	}
	log.Println("start rpc server on", l.Addr())
	addr <- l.Addr().String()
	server.Accept(l)
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	addr := make(chan string)
	go startServer(addr)

	// in fact, following code is like a simple geerpc client
	conn, _ := net.Dial("tcp", <-addr)
	defer func() { _ = conn.Close() }()

	time.Sleep(time.Second)
	// send options
	_ = json.NewEncoder(conn).Encode(server.DefaultOption)
	cc := codec.NewGobCodec(conn)
	// send request & receive response
	for i := 0; i < 5; i++ {
		h := &codec.Header{
			ServiceMethod: "Foo.Sum",
			SequenceNum:   uint64(i),
		}
		_ = cc.Write(h, fmt.Sprintf("geerpc req %d", h.SequenceNum))
		_ = cc.ReadHeader(h)
		var reply string
		_ = cc.ReadBody(&reply)
		log.Println("reply:", reply)
	}
}
