package main

import (
	"flag"
	"net"
	"net/rpc"
)

var t = false

var (
	generateKey      = flag.Bool("key-gen", false, "Force controller to generate a new key immediately")
	blacklistKeyname = flag.String("blacklist", "", "Give a keyname to blacklist")

	generated = &t
)

func main() {
	flag.Parse()
	conn, err := net.Dial("tcp", ":8081")
	if err != nil {
		panic(err)
	}
	client := rpc.NewClient(conn)
	if *blacklistKeyname != "" {
		if err = client.Call("blacklister.Blacklist", *blacklistKeyname, generated); err != nil {
			panic(err)
		}
	}
	if *generateKey && !*generated {
		if err = client.Call("trigger.Trigger", struct{}{}, &struct{}{}); err != nil {
			panic(err)
		}
	}
}
