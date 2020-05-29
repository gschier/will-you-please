package main

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func startProxy(addr string, port string) error {
	remoteU, err := url.Parse(addr)
	if err != nil {
		return err
	}

	proxy := httputil.NewSingleHostReverseProxy(remoteU)
	h := http.NewServeMux()
	h.Handle("/", &proxyHandler{p: proxy})

	l := ":" + port
	fmt.Println("[wyp] Proxy listening on", l)
	return http.ListenAndServe(l, h)
}

type proxyHandler struct {
	p *httputil.ReverseProxy
}

func (ph *proxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ph.p.ServeHTTP(w, r)
}
