package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
)

var (
	keyname = flag.String("keyname", "", "Keyname to blacklist")
)

func main() {
	flag.Parse()
	if *keyname == "" {
		panic(fmt.Errorf("No keyname provided, use --keyname=<name>"))
	}
	conn, err := net.Dial("tcp", ":8081")
	if err != nil {
		panic(err)
	}
	client := rpc.NewClient(conn)
	if err = client.Call("blacklister.Blacklist", *keyname, &struct{}{}); err != nil {
		panic(err)
	}
}
