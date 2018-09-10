package reflector

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/urlfetch"
)

const (
	// A timeout of 0 means to use the App Engine default (5 seconds).
	urlFetchTimeout = 20 * time.Second
)

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

// Hop-by-hop headers. These are removed when sent to the backend.
// http://www.w3.org/Protocols/rfc2616/rfc2616-sec13.html
var stoppedHeaders = map[string]bool{
	"Whistler-Host":       true,
	"Connection":          true,
	"Keep-Alive":          true,
	"Proxy-Authenticate":  true,
	"Proxy-Authorization": true,
	"Te":                true, // canonicalized version of "TE"
	"Trailers":          true,
	"Transfer-Encoding": true,
	"Upgrade":           true,
}

// Basic client request check
func checkRequest(r *http.Request) error {
	/*if r.URL.Path != "/" && r.URL.Path != "" {
		return errors.New("Non empty path in request")
	}*/

	return nil
}

// Make a copy of r, removing the headers in stoppedHeaders, getting next host from header and checking
// if it is allowed
func copyRequest(context context.Context, r *http.Request) (*http.Request, error) {
	fwURL := r.Header.Get("Whistler-Host")
	if fwURL == "" {
		return nil, errors.New("No Host header in request")
	}

	fwURL, err := GetWhistlerURL(fwURL)
	if err != nil {
		return nil, err
	}

	c, err := http.NewRequest(r.Method, fwURL, r.Body)
	if err != nil {
		return nil, err
	}

	for key, values := range r.Header {
		if stoppedHeaders[key] == false {
			for _, value := range values {
				c.Header.Add(key, value)
			}
		}
	}

	return c, nil
}

func handler(w http.ResponseWriter, r *http.Request) {
	context := appengine.NewContext(r)

	err := checkRequest(r)
	if err != nil {
		log.Errorf(context, "checkRequest: %s", err)
		http.Error(w, "", http.StatusNotFound)
		return
	}

	fr, err := copyRequest(context, r)
	if err != nil {
		log.Errorf(context, "copyRequest: %s", err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	// Use urlfetch.Transport directly instead of urlfetch.Client because we
	// want only a single HTTP transaction, not following redirects.
	transport := urlfetch.Transport{
		Context: context,
		// Despite the name, Transport.Deadline is really a timeout and
		// not an absolute deadline as used in the net package. In
		// other words it is a time.Duration, not a time.Time.
		///Deadline: urlFetchTimeout, TODO: replace this..
	}
	resp, err := transport.RoundTrip(fr)
	if err != nil {
		log.Errorf(context, "RoundTrip: %s", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	copyHeader(w.Header(), resp.Header)

	w.WriteHeader(resp.StatusCode)

	_, err = io.Copy(w, resp.Body)
	if err != nil {
		log.Errorf(context, "io.Copy: %s", err)
	}
}

func init() {
	http.HandleFunc("/", handler)
}
