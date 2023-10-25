package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

// Handler serves http requests.
// It is also responsible for triggering scale-from-zero.
type Handler struct {
	Scaler    *ScalerManager
	Endpoints *EndpointsManager
	FIFO      *FIFOQueueManager
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("request: %v", r.URL)

	w.Header().Set("X-Proxy", "lingo")

	// TODO: Grab model from request body.
	modelName := r.Header.Get("X-Model")
	log.Println("model:", modelName)

	go h.Scaler.AtLeastOne(modelName)

	log.Println("Waiting for IPs")
	ips := h.Endpoints.GetIPs(r.Context(), modelName)
	log.Println("Got IPs (%v)", len(ips))

	log.Println("Entering queue")
	unqueue := h.FIFO.Enqueue(modelName)
	log.Println("Admitted into queue")
	defer unqueue()

	// TODO: Avoid creating new reverse proxies for each request.
	// TODO: Don't just all the first IP every time... round robin or something.
	newReverseProxy(ips[0]).ServeHTTP(w, r)
}

func newReverseProxy(ip string) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(&url.URL{
		Scheme: "http",
		Host:   ip + ":80",
	})
	return proxy
}
