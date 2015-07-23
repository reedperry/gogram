package gogram

import (
	"appengine"
	"appengine/user"

	"github.com/gorilla/context"

	"errors"
	"fmt"
	"net/http"
)

// Authorize wraps a Handler to run authorization before executing it. If authorization fails,
// the user will either be sent to a login page, or receive a 403 Forbidden response.
func Authorize(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := appengine.NewContext(r)
		_, err := authorize(r, c)
		if err != nil {
			// TODO Really need to decide this based on whether the user is attempting to view a
			// page or calling for a raw data response
			if r.URL.Path == "/" {
				loginURL, _ := user.LoginURL(c, "/")
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				fmt.Fprintf(w, `You are not signed in! Sign in <a href="%s">here</a>.`, loginURL)
			} else {
				c.Infof("User authorization failed: %v", err)
				http.Error(w, "Not Authorized!", http.StatusForbidden)
			}

			return
		}

		h.ServeHTTP(w, r)
	})
}

// Verify that the user making the request is signed in and authorized to continue.
// The user is returned if signed in and authorized, otherwise an error is returned
// with a nil user value.
// When the user is signed in, this function stores the user in the request context.
func authorize(r *http.Request, c appengine.Context) (*user.User, error) {
	u := user.Current(c)
	if u == nil {
		return nil, errors.New("Authorization failed. User not logged in.")
	}

	context.Set(r, UserCtxKey, u)

	return u, nil
}
