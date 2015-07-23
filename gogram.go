package gogram

import (
	"net/http"
)

func init() {
	http.Handle("/", Authorize(Router()))
}
