package pillar

import (
	"net/url"
)

var Conf struct {
	DatabaseName string `default:"eve"`
	DatabaseHost string `default:"localhost:27017"`
	Listen       string `default:":5002"`
	Origin       string // "protocol://hostname:port" of iframe-embedding server.
	TLSKey       string
	TLSCert      string
	HttpForward  string
}

type ProxyConf struct {
	ClientIPHeader string
	Target         url.URL
}
