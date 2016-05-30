package proxy

import (
	"errors"
	"github.com/armadillica/pillar-notifserv/pillar"
	"net"
	"net/http"
	"net/url"
)

type Target struct {
	// URL is the endpoint the service instance listens on
	URL *url.URL
}

// addHeaders adds/updates headers in request
//
// * add/update `Forwarded` header
// * add X-Forwarded-Proto header, if not present
// * add X-Real-Ip, if not present
// * ClientIPHeader != "": Set header with that name to <remote ip>
// * TLS connection: Set header with name from `cfg.TLSHeader` to `cfg.TLSHeaderValue`
//
func addHeaders(r *http.Request, cfg pillar.ProxyConf) error {
	remoteIP, remotePort, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return errors.New("cannot parse " + r.RemoteAddr)
	}

	if r.Header.Get("X-Real-Ip") == "" {
		r.Header.Set("X-Real-Ip", remoteIP)
	}

	// set the X-Forwarded-For header for websocket
	// connections since they aren't handled by the
	// http proxy which sets it.
	r.Header.Set("X-Forwarded-For", remoteIP)

	ws := r.Header.Get("Upgrade") == "websocket"
	if r.Header.Get("X-Forwarded-Proto") == "" {
		switch {
		case ws && r.TLS != nil:
			r.Header.Set("X-Forwarded-Proto", "wss")
		case ws && r.TLS == nil:
			r.Header.Set("X-Forwarded-Proto", "ws")
		case r.TLS != nil:
			r.Header.Set("X-Forwarded-Proto", "https")
		default:
			r.Header.Set("X-Forwarded-Proto", "http")
		}
	}

	if r.Header.Get("X-Forwarded-Port") == "" {
		r.Header.Set("X-Forwarded-Port", remotePort)
	}

	fwd := r.Header.Get("Forwarded")
	if fwd == "" {
		fwd = "for=" + remoteIP
		switch {
		case ws && r.TLS != nil:
			fwd += "; proto=wss"
		case ws && r.TLS == nil:
			fwd += "; proto=ws"
		case r.TLS != nil:
			fwd += "; proto=https"
		default:
			fwd += "; proto=http"
		}
	}
	r.Header.Set("Forwarded", fwd)

	return nil
}
