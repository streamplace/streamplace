package main

import (
	"crypto/rand"
	"fmt"
	"os"
	"time"

	iroh "stream.place/streamplace/pkg/iroh/generated/iroh_streamplace"
	_ "stream.place/streamplace/pkg/streamplacedeps"
)

func panicIfErr(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	tickets := os.Args[1:]

	secret := make([]byte, 32)
	_, err := rand.Read(secret)
	panicIfErr(err)

	fmt.Println("Starting with tickets", tickets)
	config := iroh.Config{
		Key:             secret,
		Topic:           make([]byte, 32), // all zero topic for testing
		MaxSendDuration: 1000_000_000,     // 1s
	}
	fmt.Printf("Config created %+v\n", config)
	node, err := iroh.NodeSender(config)
	panicIfErr(err)

	db := node.Db()
	w := node.NodeScope()

	node_id, err := node.NodeId()
	panicIfErr(err)
	fmt.Println("Node ID:", node_id)

	ticket, err := node.Ticket()
	panicIfErr(err)
	fmt.Println("Ticket:", ticket)

	if len(tickets) > 0 {
		err = node.JoinPeers(tickets)
		panicIfErr(err)
	}

	err = w.Put(nil, []byte("hello"), []byte("world"))
	panicIfErr(err)
	stream := []byte("stream1")
	err = w.Put(&stream, []byte("subscribed"), []byte("true"))
	panicIfErr(err)

	filter := iroh.NewFilter()
	items, err := db.IterWithOpts(filter)
	panicIfErr(err)
	fmt.Printf("Iter items: %+v\n", items)

	filter2 := iroh.NewFilter().Global()
	items2, err := db.IterWithOpts(filter2)
	panicIfErr(err)
	fmt.Printf("Iter items: %+v\n", items2)

	filter3 := iroh.NewFilter().Stream(stream)
	items3, err := db.IterWithOpts(filter3)
	panicIfErr(err)
	fmt.Printf("Iter items: %+v\n", items3)

	go func() {
		time.Sleep(5 * time.Second)
		err := node.Shutdown()
		panicIfErr(err)
	}()

	sub := db.Subscribe(iroh.NewFilter())
	for {
		ev, err := sub.NextRaw()
		panicIfErr(err)
		if ev == nil {
			fmt.Println("Subscription closed")
			break
		}
		switch (*ev).(type) {
		case iroh.SubscribeItemEntry:
			fmt.Printf("%+v\n", (*ev).(iroh.SubscribeItemEntry))
		case iroh.SubscribeItemCurrentDone:
			fmt.Printf("Got current done event: %+v\n", (*ev).(iroh.SubscribeItemCurrentDone))
		case iroh.SubscribeItemExpired:
			fmt.Printf("Got expired event: %+v\n", (*ev).(iroh.SubscribeItemExpired))
		case iroh.SubscribeItemOther:
			fmt.Printf("Got other event: %+v\n", (*ev).(iroh.SubscribeItemOther))
		}
	}
}
