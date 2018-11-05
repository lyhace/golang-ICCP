package main

import (
	"flag"
	. "github.com/0x19/goesl"
)

func main() {

	var (
		fshost   = flag.String("fshost", "172.16.40.133", "Freeswitch hostname. Default: localhost")
		fsport   = flag.Uint("fsport", 8021, "Freeswitch port. Default: 8021")
		password = flag.String("pass", "ClueCon", "Freeswitch password. Default: ClueCon")
		timeout  = flag.Int("timeout", 10, "Freeswitch conneciton timeout in seconds. Default: 10")
	)

	client, err := NewClient(*fshost, *fsport, *password, *timeout)

	if err != nil {
		Error("Error while creating new client: %s", err)
		return
	}

	// Apparently all is good... Let us now handle connection :)
	// We don't want this to be inside of new connection as who knows where it my lead us.
	// Remember that this is crutial part in handling incoming messages. This is a must!
	go client.Handle()
}