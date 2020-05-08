package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/satta/ifplugo"
	log "github.com/sirupsen/logrus"
)

func main() {
	deltaValPtr := flag.Int("delta", 0, "number of bytes transferred per polling period to mark interface as seeing traffic")
	pollingPtr := flag.Duration("poll", 2*time.Second, "polling period")
	verbPtr := flag.Bool("verbose", false, "verbose logging")
	flag.Parse()

	if len(flag.Args()) < 1 {
		fmt.Println("Usage: ifplugo-status <iface1,iface2,...>")
		os.Exit(1)
	}
	ifaces := strings.Split(flag.Args()[0], ",")

	if *verbPtr {
		log.SetLevel(log.DebugLevel)
	}

	outchan := make(chan ifplugo.LinkStatusSample)
	mon := ifplugo.MakeLinkStatusMonitor(*pollingPtr, ifaces, outchan)
	if *deltaValPtr > 0 {
		mon.CheckIncomingDelta(true, uint64(*deltaValPtr))
	}
	go func() {
		for v := range outchan {
			for k, v := range v.Ifaces {
				fmt.Printf("%s: %s\n", k, v)
			}
		}
	}()
	mon.Run()
	select {}
}
