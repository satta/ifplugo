package ifplugo

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

/*
#cgo LDFLAGS: -ldaemon
#include <interface.h>
*/
import (
	"C"
)

import (
	"syscall"
	"time"

	"github.com/shirou/gopsutil/net"
	log "github.com/sirupsen/logrus"
)

// InterfaceStatus represents the link status of an interface.
type InterfaceStatus int

const (
	// InterfaceUnknown represents an interface with no assigned state.
	InterfaceUnknown InterfaceStatus = iota
	// InterfaceUp represents an interface with a cable connected.
	InterfaceUp
	// InterfaceDown represents an interface with no cable connected.
	InterfaceDown
	// InterfaceErr represents an interface with errors querying its status.
	InterfaceErr
)

var statusLookup = map[C.interface_status_t]InterfaceStatus{
	C.IFSTATUS_UP:   InterfaceUp,
	C.IFSTATUS_DOWN: InterfaceDown,
	C.IFSTATUS_ERR:  InterfaceErr,
}

func (s InterfaceStatus) String() string {
	switch s {
	case InterfaceUp:
		return "link"
	case InterfaceDown:
		return "no link"
	case InterfaceErr:
		return "error"
	default:
		return "unknown"
	}
}

// GetLinkStatus returns, for a given interface, the corresponding status code
// at the time of the call. If any error was encountered (e.g. invalid
// interface, etc.) we simply return ifplugo.InterfaceErr.
func GetLinkStatus(iface string) (InterfaceStatus, error) {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM,
		syscall.IPPROTO_IP)
	if err != nil {
		return InterfaceErr, err
	}
	defer syscall.Close(fd)

	e := C.interface_detect_beat_ethtool(C.int(fd), C.CString(iface))
	if e == C.IFSTATUS_ERR {
		e = C.interface_detect_beat_mii(C.int(fd), C.CString(iface))
		if e == C.IFSTATUS_ERR {
			e = C.interface_detect_beat_wlan(C.int(fd), C.CString(iface))
			if e == C.IFSTATUS_ERR {
				e = C.interface_detect_beat_iff(C.int(fd), C.CString(iface))
			}
		}
	}

	return statusLookup[e], nil
}

// LinkStatusMonitor represents a concurrent software component that
// periodically checks a list of given interfaces and returns their link status
// via a specified channel.
type LinkStatusMonitor struct {
	PollPeriod             time.Duration
	LastStatus             map[string]InterfaceStatus
	LastStats              map[string]net.IOCountersStat
	checkIncomingDelta     bool
	checkIncomingThreshold uint64
	OutChan                chan LinkStatusSample
	CloseChan              chan bool
	ClosedChan             chan bool
	Ifaces                 []string
}

// LinkStatusSample is a single description of the link status at a given time.
// Changed is set to true if the state is different than the previously emitted
// one.
type LinkStatusSample struct {
	Ifaces map[string]InterfaceStatus
}

// MakeLinkStatusMonitor creates a new LinkStatusMonitor, polling each interval
// given in pollPeriod for the status information of the interfaces given in
// ifaces and outputting results as a map of interface->status pairs in the
// channel outChan.
func MakeLinkStatusMonitor(pollPeriod time.Duration, ifaces []string,
	outChan chan LinkStatusSample) *LinkStatusMonitor {
	a := &LinkStatusMonitor{
		PollPeriod: pollPeriod,
		OutChan:    outChan,
		CloseChan:  make(chan bool),
		ClosedChan: make(chan bool),
		Ifaces:     ifaces,
		LastStatus: make(map[string]InterfaceStatus),
		LastStats:  make(map[string]net.IOCountersStat),
	}
	return a
}

// CheckIncomingDelta allows to enable the optional behaviour to also consider
// an interface as 'up' if traffic is received on it. This is, for example,
// necessary in passive monitoring setups where there is no physical link
// detected (e.g. using taps that only provide RX lines).
func (a *LinkStatusMonitor) CheckIncomingDelta(val bool, threshold uint64) {
	a.checkIncomingDelta = val
	a.checkIncomingThreshold = threshold
}

func myDiffAbs(new, old uint64) uint64 {
	if new > old {
		return new - old
	}
	return 0
}

func (a *LinkStatusMonitor) flush() error {
	out := LinkStatusSample{
		Ifaces: make(map[string]InterfaceStatus),
	}

	changed := false
	for _, iface := range a.Ifaces {
		v, err := GetLinkStatus(iface)
		if err != nil {
			out.Ifaces[iface] = InterfaceUnknown
		}
		out.Ifaces[iface] = v

		if a.LastStatus[iface] != out.Ifaces[iface] {
			changed = true
			a.LastStatus[iface] = out.Ifaces[iface]
		}
	}

	ifstats, err := net.IOCounters(true)
	if err != nil {
		return err
	}

	if a.checkIncomingDelta {
		for _, stat := range ifstats {
			for _, iface := range a.Ifaces {
				if stat.Name == iface {
					log.Debugf("%s, %d/%d -> %d", a.LastStatus[iface], stat.BytesRecv, a.LastStats[iface].BytesRecv, myDiffAbs(stat.BytesRecv, a.LastStats[iface].BytesRecv))
					if a.LastStatus[iface] != InterfaceUp {
						if myDiffAbs(stat.BytesRecv, a.LastStats[iface].BytesRecv) > a.checkIncomingThreshold {
							out.Ifaces[iface] = InterfaceUp
							log.Debugf("changed %s to up", iface)
						} else {
							out.Ifaces[iface] = a.LastStatus[iface]
						}
					} else {
						if myDiffAbs(stat.BytesRecv, a.LastStats[iface].BytesRecv) <= a.checkIncomingThreshold {
							out.Ifaces[iface] = InterfaceDown
							log.Debugf("changed %s to down", iface)
						} else {
							out.Ifaces[iface] = a.LastStatus[iface]
						}
					}
					if a.LastStatus[iface] != out.Ifaces[iface] {
						log.Debugf("%s <-> %s", a.LastStatus[iface], out.Ifaces[iface])
						changed = true
						a.LastStatus[iface] = out.Ifaces[iface]
					}
				}
			}
			a.LastStats[stat.Name] = stat
		}
	}

	if changed {
		a.OutChan <- out
	}
	return nil
}

// Run starts watching interfaces in the background.
func (a *LinkStatusMonitor) Run() {
	go func() {
		a.flush()
		for {
			select {
			case <-a.CloseChan:
				close(a.ClosedChan)
				return
			case <-time.After(a.PollPeriod):
				a.flush()
			}
		}
	}()
}

// Stop causes the monitor to cease monitoring interfaces.
func (a *LinkStatusMonitor) Stop() {
	close(a.CloseChan)
	<-a.ClosedChan
}
