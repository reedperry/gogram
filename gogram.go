package gogram

import (
	"appengine"

	"fmt"
	"net/http"

	"github.com/reedperry/gogram/api"
)

// Temporary...testing duplicate ID generation
func GenerateIDs(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	set := make([]string, 1000)

	for i := 0; i < 1000; i++ {
		id, err := api.NewUID(c)
		if err != nil {
			continue
		}

		if setContains(set, id) {
			c.Errorf("DUPLICATE FOUND AFTER %v: %v", i, id)
		}
		set[i] = id

		fmt.Fprint(w, id+"\n")
	}
}

func setContains(set []string, item string) bool {
	for _, member := range set {
		if member == item {
			return true
		}
	}

	return false
}
