package gogram

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/reedperry/gogram/middleware"
)

func init() {
	http.Handle("/", middleware.Authorize(Router()))
}

func ServeApp(w http.ResponseWriter, r *http.Request) {
	content, err := ioutil.ReadFile("static/index.html")
	if err != nil {
		fmt.Fprint(w, "index.html not found!")
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, string(content))
}
