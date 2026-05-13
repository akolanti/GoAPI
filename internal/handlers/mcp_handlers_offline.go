//go:build offline

package handlers

import "net/http"

func MCPHandler(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}

func MCPStatusHandler(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}
