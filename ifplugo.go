package ifplugo

import (
	"syscall"
	"time"
	"unsafe"

	"github.com/shirou/gopsutil/net"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
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

const (
	ethtoolGlink = 0x0000000a
	siocGiwap    = 0x8B15
)

type ifReq struct {
	Name [unix.IFNAMSIZ]byte
	Data uintptr
}

type iwReqApAddr struct {
	_      uint16
	saData [14]byte
}

type iwReq struct {
	Name [unix.IFNAMSIZ]byte
	Data iwReqApAddr
}

type ethtoolValue struct {
	Cmd  uint32
	Data uint32
}

type miiIoctl struct {
	Phyid  uint16
	Regnum uint16
	Valin  uint16
	Valout uint16
}

func detectBeatEthtool(fd int, iface string) (bool, error) {
	ev := ethtoolValue{
		Cmd: ethtoolGlink,
	}
	ifreq := ifReq{
		Data: uintptr(unsafe.Pointer(&ev)),
	}
	copy(ifreq.Name[:], iface)

	_, _, errno := syscall.RawSyscall(syscall.SYS_IOCTL, uintptr(fd),
		uintptr(unix.SIOCETHTOOL), uintptr(unsafe.Pointer(&ifreq)))
	if errno != 0 {
		return false, errno
	}

	return ev.Data != 0, nil
}

func detectBeatMII(fd int, iface string) (bool, error) {
	ifreq := ifReq{}
	copy(ifreq.Name[:], iface)

	_, _, errno := syscall.RawSyscall(syscall.SYS_IOCTL, uintptr(fd),
		uintptr(unix.SIOCGMIIPHY), uintptr(unsafe.Pointer(&ifreq)))
	if errno != 0 {
		return false, errno
	}
	miioctl := (*miiIoctl)(unsafe.Pointer(&ifreq.Data))
	miioctl.Regnum = 1

	_, _, errno = syscall.RawSyscall(syscall.SYS_IOCTL, uintptr(fd),
		uintptr(unix.SIOCGMIIREG), uintptr(unsafe.Pointer(&ifreq)))
	if errno != 0 {
		return false, errno
	}
	miioctl = (*miiIoctl)(unsafe.Pointer(&ifreq.Data))

	return (miioctl.Valout & 0x0004) != 0, nil
}

func detectBeatIff(fd int, iface string) (bool, error) {
	ifreq := ifReq{}
	copy(ifreq.Name[:], iface)

	_, _, errno := syscall.RawSyscall(syscall.SYS_IOCTL, uintptr(fd),
		uintptr(unix.SIOCGIFFLAGS), uintptr(unsafe.Pointer(&ifreq)))
	if errno != 0 {
		return false, errno
	}

	return (ifreq.Data & unix.IFF_RUNNING) != 0, nil
}

func macIsSet(in []byte) bool {
	b := 1

	for i := 1; i < len(in); i++ {
		if in[i] != in[0] {
			b = 0
			break
		}
	}

	return b == 0 || (in[0] != 0xFF && in[0] != 0x44 && in[0] != 0x00)
}

func detectBeatWifi(fd int, iface string) (bool, error) {
	iwreq := iwReq{}
	copy(iwreq.Name[:], iface)

	_, _, errno := syscall.RawSyscall(syscall.SYS_IOCTL, uintptr(fd),
		uintptr(siocGiwap), uintptr(unsafe.Pointer(&iwreq)))
	if errno != 0 {
		return false, errno
	}

	return macIsSet(iwreq.Data.saData[:6]), nil
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

	e, err := detectBeatEthtool(fd, iface)
	if err != nil {
		e, err = detectBeatMII(fd, iface)
		if err != nil {
			e, err = detectBeatWifi(fd, iface)
			if err != nil {
				e, err = detectBeatIff(fd, iface)
			}
		}
	}

	if err != nil {
		return InterfaceErr, err
	}
	if e {
		return InterfaceUp, nil
	}
	return InterfaceDown, nil
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
	configuredByLink       map[string]bool
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
		PollPeriod:       pollPeriod,
		OutChan:          outChan,
		CloseChan:        make(chan bool),
		ClosedChan:       make(chan bool),
		Ifaces:           ifaces,
		LastStatus:       make(map[string]InterfaceStatus),
		LastStats:        make(map[string]net.IOCountersStat),
		configuredByLink: make(map[string]bool),
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

	// try to get status via link
	for _, iface := range a.Ifaces {
		v, err := GetLinkStatus(iface)
		if err != nil {
			out.Ifaces[iface] = InterfaceUnknown
		}
		out.Ifaces[iface] = v
		if v == InterfaceUp {
			// this interface has been seen up once via actual link status
			// let's record this fact so we won't override this from data
			// flow info
			if _, ok := a.configuredByLink[iface]; !ok {
				a.configuredByLink[iface] = true
			}
		}
		log.Debug("link status: ", iface, v)
	}

	// also try to determine status from data flow
	if a.checkIncomingDelta {
		ifstats, err := net.IOCounters(true)
		if err != nil {
			return err
		}
		for _, stat := range ifstats {
			for _, iface := range a.Ifaces {
				if stat.Name == iface {
					if _, ok := a.configuredByLink[iface]; ok {
						if a.configuredByLink[iface] {
							continue
						}
					}
					log.Debugf("%s, %s, %d/%d -> %d", iface, a.LastStatus[iface], stat.BytesRecv, a.LastStats[iface].BytesRecv, myDiffAbs(stat.BytesRecv, a.LastStats[iface].BytesRecv))
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
					a.LastStats[iface] = stat
				}
			}
		}
	}

	changed := false
	for iface := range out.Ifaces {
		if a.LastStatus[iface] != out.Ifaces[iface] {
			changed = true
			log.Debugf("status changed %s <-> %s", a.LastStatus[iface], out.Ifaces[iface])
			a.LastStatus[iface] = out.Ifaces[iface]
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
