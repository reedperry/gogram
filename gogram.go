package gogram

import (
	"net/http"
	"time"
)

func init() {
	http.Handle("/", Authorize(Router()))
}
