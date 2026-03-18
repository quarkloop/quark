package httpserver

import "net/http"

// Router defines an HTTP routing module that can register its endpoints to a multiplexer.
type Router interface {
	RegisterRoutes(mux *http.ServeMux)
}
