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
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/satta/ifplugo"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: ifplugo-status <iface1,iface2,...>")
		os.Exit(1)
	}
	ifaces := strings.Split(os.Args[1], ",")

	outchan := make(chan map[string]ifplugo.InterfaceStatus)
	mon := ifplugo.MakeLinkStatusMonitor(2*time.Second, ifaces, outchan)
	go func() {
		for v := range outchan {
			for k, v := range v {
				fmt.Printf("%s: %d\n", k, v)
			}
		}
	}()
	mon.Run()
	select {}
}
