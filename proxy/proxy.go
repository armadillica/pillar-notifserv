package proxy

import (
	"net/http"

	"github.com/armadillica/pillar-notifserv/pillar"
)

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

	url := p.cfg.Target.ResolveReference(r.URL)
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
