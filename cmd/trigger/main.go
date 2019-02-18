package main

import (
	"net"
	"net/rpc"
)

func main() {
	conn, err := net.Dial("tcp", ":8081")
	if err != nil {
		panic(err)
	}
	client := rpc.NewClient(conn)
	if err = client.Call("trigger.Trigger", struct{}{}, &struct{}{}); err != nil {
		panic(err)
	}
}
