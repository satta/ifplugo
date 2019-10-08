# ifplugo

[![GoDoc](https://godoc.org/github.com/satta/ifplugo?status.svg)](http://godoc.org/github.com/satta/ifplugo)
[![CircleCI](https://circleci.com/gh/satta/ifplugo.svg?style=shield)](https://circleci.com/gh/satta/ifplugo)

ifplugo delivers network interface link information and link changes. It does this (on Linux) by using code from [ifplugd](http://0pointer.de/lennart/projects/ifplugd/) to gather the necessary status information, then emits a status summary on a given channel. This summary (`LinkStatusSample`) is emitted on the first invocation and each time the state changes for at least one monitored interface.

```Go
type LinkStatusSample struct {
    Ifaces  map[string]InterfaceStatus
}
```

where `InterfaceStatus` can be:

```Go
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
```

These summaries can then easily be consumed (example):

```Go
outchan := make(chan ifplugo.LinkStatusSample)
mon := ifplugo.MakeLinkStatusMonitor(2 * time.Second, []string{"eth0"}, outchan)
go func() {
    for v := range outchan {
        for k, v := range v.Ifaces {
            fmt.Printf("%s: %s\n", k, v)
        }
    }
}()
mon.Run()
```

It is also possible to determine the status of an interface from whether any data is flowing or not. This can be useful if, for example, the interesting interface is only connected to one way of the physical connection (RX or TX) or for other reasons can not complete autonegotiation. Use `CheckIncomingDelta()` in this case, it allows to also mark an interface as 'up' and seeing traffic if a certain threshold of received bytes is exceeded during one polling period. Example:

```Go
mon.CheckIncomingDelta(true, 1000)
```

This would, for example, also mark an interface as up if more than 1000 bytes are received during the polling period, and mark the interface as down if there are ever less than 1000 bytes received in a polling period.

## Prerequisites

To build ifplugo, one needs Go and C compilers.
Also, obviously, this is Linux-only.

## Example

See the source code of the simple command line tools in `cmd/*` for more simple examples of how to use ifplugo.

```Text
$ ifplugo-watch eth0,eth1,eth2,eth3
eth0: link
eth1: link
eth2: link
eth3: no link
^C
$
```

## Authors

This source code includes parts of ifplugd, written by Lennart Poettering <mzvscyhtq (at) 0pointer (dot) de>.

The Go component of the code was written by Sascha Steinbiss <sascha (at) steinbiss (dot) name>.

## License

GPL2
