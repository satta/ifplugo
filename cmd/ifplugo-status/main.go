package main

import (
	"fmt"
	"os"

	"github.com/satta/ifplugo"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: ifplugo-status <iface>")
		os.Exit(1)
	}
	fmt.Println(ifplugo.GetLinkStatus(os.Args[1]))
}
