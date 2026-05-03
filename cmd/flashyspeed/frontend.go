package main

import (
	"io/fs"
	"net/http"

	flashyspeed "github.com/flashyspeed/flashyspeed"
)

func serveFrontend() http.HandlerFunc {
	dist, err := fs.Sub(flashyspeed.WebDist, "web/dist")
	if err != nil {
		panic("embed: " + err.Error())
	}
	fsHandler := http.FileServer(http.FS(dist))

	return func(w http.ResponseWriter, r *http.Request) {
		// Try to open the requested path; if not found, serve index.html for SPA routing
		_, err := dist.Open(r.URL.Path[1:]) // strip leading /
		if err != nil {
			// Serve index.html for all unknown paths (client-side routing).
			// Deep-copy the URL so we don't mutate the original request.
			r2 := *r
			u2 := *r.URL
			u2.Path = "/"
			r2.URL = &u2
			fsHandler.ServeHTTP(w, &r2)
			return
		}
		fsHandler.ServeHTTP(w, r)
	}
}
