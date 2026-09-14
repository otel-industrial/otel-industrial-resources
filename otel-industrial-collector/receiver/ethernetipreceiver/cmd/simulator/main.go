// Command simulator runs a minimal EtherNet/IP (CIP) server for local testing
// of the ethernetipreceiver without requiring real PLC hardware.
//
// It listens on the standard EtherNet/IP ports (0.0.0.0:44818 TCP,
// 0.0.0.0:2222 UDP), matching gologix.Server's fixed bind behavior.
package main

import (
	"log"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/danomagnum/gologix"
)

func main() {
	router := gologix.NewRouter()
	tags := &gologix.MapTagProvider{
		Data: map[string]any{
			"testtag":     1.0,
			"temperature": 68.0,
			"pumpstatus":  int32(1),
		},
	}

	path, err := gologix.ParsePath("1,0")
	if err != nil {
		log.Fatalf("parsing path: %v", err)
	}
	router.Handle(path.Bytes(), tags)

	server := gologix.NewServer(router)

	go func() {
		log.Println("EtherNet/IP simulator listening on 0.0.0.0:44818 (TCP) / 0.0.0.0:2222 (UDP)")
		if err := server.Serve(); err != nil {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Slowly drift tag values so polling shows changing data over time.
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	t := 0.0
	go func() {
		for range ticker.C {
			t += 0.1
			tags.Mutex.Lock()
			tags.Data["temperature"] = 68.0 + 5*math.Sin(t)
			tags.Data["testtag"] = 1.0 + 0.5*math.Cos(t)
			tags.Mutex.Unlock()
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("shutting down simulator")
}
