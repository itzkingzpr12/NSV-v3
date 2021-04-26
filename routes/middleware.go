package routes

import "net/http"

// ResponseWriter struct
type ResponseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	message     []byte
}

func wrapResponseWriter(w http.ResponseWriter) *ResponseWriter {
	return &ResponseWriter{ResponseWriter: w}
}

// Status func
func (rw *ResponseWriter) Status() int {
	return rw.status
}

// WriteHeader func
func (rw *ResponseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}

	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
	rw.wroteHeader = true

	return
}

// Message func
func (rw *ResponseWriter) Message() []byte {
	return rw.message
}

// WriteMessage func
func (rw *ResponseWriter) Write(message []byte) (int, error) {
	rw.message = message

	return rw.ResponseWriter.Write(message)
}
