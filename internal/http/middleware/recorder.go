package middleware

import (
	"io"
	"net/http"
)

// recorder remembers what a handler did to the response.
//
// Logger needs the status and size for its completion line, and recover needs to know
// whether the response has already started before it tries to write a 500 over
// the top of it.
type recorder struct {
	http.ResponseWriter

	status  int
	written int64
	wrote   bool
}

// wrap returns the recorder already installed on w, or installs one.
func wrap(w http.ResponseWriter) *recorder {
	if rec, ok := w.(*recorder); ok {
		return rec
	}
	return &recorder{ResponseWriter: w, status: http.StatusOK}
}

func (r *recorder) WriteHeader(status int) {
	if r.wrote {
		return
	}
	r.wrote = true
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *recorder) Write(b []byte) (int, error) {
	if !r.wrote {
		r.WriteHeader(http.StatusOK)
	}
	n, err := r.ResponseWriter.Write(b)
	r.written += int64(n)
	return n, err
}

func (r *recorder) ReadFrom(src io.Reader) (int64, error) {
	if !r.wrote {
		r.WriteHeader(http.StatusOK)
	}
	n, err := io.Copy(r.ResponseWriter, src)
	r.written += n
	return n, err
}

func (r *recorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }
