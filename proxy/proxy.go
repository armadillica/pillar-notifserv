package proxy

import (
	"net/http"
	"net/url"
	"github.com/armadillica/pillar-notifserv/pillar"
)

var root_url, _ = url.Parse("/")

// Proxy is a dynamic reverse proxy.
type Proxy struct {
	tr  http.RoundTripper
	cfg pillar.ProxyConf
}

func New(tr http.RoundTripper, cfg pillar.ProxyConf) *Proxy {
	return &Proxy{
		tr:  tr,
		cfg: cfg,
	}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if ShuttingDown() {
		http.Error(w, "shutting down", http.StatusServiceUnavailable)
		return
	}

	if err := addHeaders(r, p.cfg); err != nil {
		http.Error(w, "cannot parse "+r.RemoteAddr, http.StatusInternalServerError)
		return
	}

	// Without this hack of using a / URL, proxied requests
	// are doubled, i.e. a request to /jemoeder will be sent
	// to http://proxytarget/jemoeder/jemoeder.
	url := p.cfg.Target.ResolveReference(root_url)
	var h http.Handler
	switch {
	case r.Header.Get("Upgrade") == "websocket":
		h = newRawProxy(url)

		// To use the filtered proxy use
		// h = newWSProxy(url)
	default:
		h = newHTTPProxy(url, p.tr)
	}

	h.ServeHTTP(w, r)
}
