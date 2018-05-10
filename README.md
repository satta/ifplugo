# ifplugo

ifplugo delivers network interface link information and link changes. It does this (on Linux) by using code from [ifplugd](http://0pointer.de/lennart/projects/ifplugd/) to gather the necessary status information, then emits periodic status summaries on a given channel. This summary (`LinkStatusSample`) specifies whether the link status has changed and what the current state of each interface is:

```Go
type LinkStatusSample struct {
	Changed bool
	Ifaces  map[string]InterfaceStatus
}
```

These summaries can then easily be consumed (example):

```Go
outchan := make(chan ifplugo.LinkStatusSample)
mon := ifplugo.MakeLinkStatusMonitor(2 * time.Second, []string{"eth0"}, outchan)
go func() {
    for v := range outchan {
        fmt.Println("changed: ", v.Changed)
        for k, v := range v.Ifaces {
            fmt.Printf("%s: %d\n", k, v)
        }
    }
}()
mon.Run()
```

### Prerequisites

To build ifplugo, one needs [libdaemon](http://0pointer.de/lennart/projects/libdaemon/) in addition to Go and C compilers.
Also, obviously, this is Linux-only.

## Example

See the simple command line tools in `cmd/*` for more examples of how to use ifplugo.

## Authors

This source code includes parts of ifplugd, written by Lennart Poettering 
<mzvscyhtq (at) 0pointer (dot) de>.

The Go component of the code was written by Sascha Steinbiss 
<sascha (at) steinbiss (dot) name>.

## License

GPL2
