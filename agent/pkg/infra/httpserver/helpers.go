package httpserver

import (
	"encoding/json"
	"io"
	"net/http"
)

// WriteJSON writes a JSON response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// WriteError writes a JSON error response.
func WriteError(w http.ResponseWriter, status int, msg string) {
	WriteJSON(w, status, map[string]string{"error": msg})
}

// ProxySSE relays an SSE stream from upstreamURL to the client.
func ProxySSE(w http.ResponseWriter, r *http.Request, upstreamURL string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		WriteError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	resp, err := http.Get(upstreamURL)
	if err != nil {
		w.Write([]byte("data: {\"error\":\"upstream not reachable\"}\n\n"))
		flusher.Flush()
		return
	}
	defer resp.Body.Close()

	buf := make([]byte, 4096)
	for {
		select {
		case <-r.Context().Done():
			return
		default:
			n, err := resp.Body.Read(buf)
			if n > 0 {
				w.Write(buf[:n])
				flusher.Flush()
			}
			if err == io.EOF || err != nil {
				return
			}
		}
	}
}

// ProxyPost forwards a POST request body to upstreamURL and relays the JSON response.
// Used by the api-server to proxy chat requests to the space-runtime.
func ProxyPost(w http.ResponseWriter, r *http.Request, upstreamURL string) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, upstreamURL, r.Body)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		WriteError(w, http.StatusBadGateway, err.Error())
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
func ProxyJSON(w http.ResponseWriter, upstreamURL string) {
	resp, err := http.Get(upstreamURL)
	if err != nil {
		WriteError(w, http.StatusBadGateway, err.Error())
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
