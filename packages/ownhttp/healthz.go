package ownhttp

import (
	"fmt"
	"net/http"
)

func RegisterVanillaHealthz(mux *http.ServeMux, serviceName string) {

	// Self healthcheck — the dashboard's own status poller probes
	// nest via the same /healthz convention as every other HTTP
	// service in the catalogue, so without this it'd be perpetually
	// "offline" to itself.
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fmt.Sprintf(`{"ok":true,"service":"%s"}`, serviceName)))
	})
}
