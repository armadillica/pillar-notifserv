Pillar notification server
==========================

Proof of concept of a notification server for Pillar, written in Go.

## Install package `golang`

Note there is golang in Ubuntu but it is not up to date. An up-to-date version may be found at
[ppa:ubuntu-lxc/lxd-stable](https://launchpad.net/~ubuntu-lxc/+archive/ubuntu/lxd-stable).
Try this:

```
$ sudo add-apt-repository ppa:ubuntu-lxc/lxd-stable
$ sudo apt-get update
$ sudo apt-get install golang
```

That is all you need to get `go` working on your system. (You can use `go env GOROOT` to be sure
where the Go files are, if you're curious.) Don't forget to create your GOPATH.


## Fabio

The HTTP proxy code originated at [Fabio](https://github.com/eBay/fabio).
