package main

import "net/http"

func (a *app) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/", a.helloHandler)
	mux.HandleFunc("GET /reports/{id}/presence", a.presenceHandler)

	return mux
}
