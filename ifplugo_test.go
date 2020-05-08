package ifplugo

import (
	"log"
	"net"
	"sync"
	"testing"
	"time"
)

func TestIface(t *testing.T) {
	intfs, err := net.Interfaces()
	if err != nil {
		t.Fatal(err)
	}
	if len(intfs) == 0 {
		log.Println("No interfaces present, skipping...")
		t.SkipNow()
	}

	empty := true
	for _, intf := range intfs {
		stats, err := GetLinkStatus(intf.Name)
		if err != nil {
			continue
		}
		log.Println("Got status for ", intf.Name)

		if stats != InterfaceErr {
			empty = false
		}
	}

	if empty {
		t.Fatal("Unable to retrieve status from any interface of this system.")
	}
}

func TestMonitor(t *testing.T) {
	intfs, err := net.Interfaces()
	if err != nil {
		t.Fatal(err)
	}
	if len(intfs) == 0 {
		log.Println("No interfaces present, skipping...")
		t.SkipNow()
	}

	ifaces := make([]string, 0)
	for _, intf := range intfs {
		ifaces = append(ifaces, intf.Name)
	}

	waitChan := make(chan bool)
	outChan := make(chan LinkStatusSample)
	mon := MakeLinkStatusMonitor(2*time.Second, ifaces, outChan)

	var resMutex sync.Mutex
	cnt := 0
	results := make(map[string]int)
	go func(c *int) {
		for o := range outChan {
			resMutex.Lock()
			for k, v := range o.Ifaces {
				results[k]++
				log.Printf("got status for %s: %s", k, v)
			}
			(*c)++
			resMutex.Unlock()
		}
		close(waitChan)
	}(&cnt)

	mon.Run()
	time.Sleep(5 * time.Second)
	mon.Stop()

	resMutex.Lock()
	if cnt != 1 {
		t.Fatalf("expected 1 output, got %d", cnt)
	}
	for _, v := range ifaces {
		if results[v] == 0 {
			t.Fatalf("unseen interface %s", v)
		}
	}

	for k := range results {
		found := false
		for _, i := range ifaces {
			if i == k {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("unknown result interface %s", k)
		}
	}
	resMutex.Unlock()
	close(outChan)
	<-waitChan

}
