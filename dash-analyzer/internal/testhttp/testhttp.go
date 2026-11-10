// Package testhttp runs HTTP fixtures without sockets or upstream access.
package testhttp

import (
	"net/http"
	"net/http/httptest"
)

type transport struct{ handler http.Handler }

func (t transport) RoundTrip(r *http.Request) (*http.Response, error) {
	if err := r.Context().Err(); err != nil {
		return nil, err
	}
	w := httptest.NewRecorder()
	t.handler.ServeHTTP(w, r)
	return w.Result(), nil
}

type Server struct {
	URL     string
	handler http.Handler
}

func NewServer(h http.Handler) *Server { return &Server{URL: "http://fixture.invalid", handler: h} }
func (s *Server) Client() *http.Client { return &http.Client{Transport: transport{s.handler}} }
func (s *Server) Close()               {}
