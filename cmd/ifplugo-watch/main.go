package main

// This file is part of ifplugo.
//
// ifplugo is free software; you can redistribute it and/or modify it
// under the terms of the GNU General Public License as published by
// the Free Software Foundation; either version 2 of the License, or
// (at your option) any later version.
//
// ifplugo is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with ifplugo; if not, write to the Free Software Foundation,
// Inc., 59 Temple Place, Suite 330, Boston, MA 02111-1307 USA.

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
