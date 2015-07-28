package api

import (
	"appengine"
	"appengine/datastore"

	"github.com/gorilla/mux"

	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
)

type ctxKey int

type OkResponse struct {
	Ok bool `json:"ok"`
}

type HasCustomDatastoreKey interface {
	DSKey(appengine.Context) (*datastore.Key, error)
	DSKeyID(appengine.Context) (string, error)
}

func requestVarProvided(r *http.Request, varName string) bool {
	vars := mux.Vars(r)
	_, ok := vars[varName]
	return ok
}

func getRequestVar(r *http.Request, varName string, c appengine.Context) string {
	vars := mux.Vars(r)
	value, ok := vars[varName]

	if !ok {
		c.Warningf("No var '%v' present in request URL.", varName)
	}

	return value
}

// ReadEntity reads a JSON value into entity from a Request body.
// An error is returned if the body cannot be read into entity.
func readEntity(r *http.Request, entity interface{}) error {
	defer r.Body.Close()

	var body []byte
	body, readErr := ioutil.ReadAll(r.Body)
	if readErr != nil {
		return readErr
	}

	err := json.Unmarshal(body, entity)
	if err != nil {
		return errors.New("Couldn't get valid JSON object from request body.")
	}

	return nil
}

func sendJsonResponse(w http.ResponseWriter, resp interface{}) {
	body, err := json.Marshal(resp)
	if err != nil {
		handleError(w, err, nil)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}

// HandleError logs and returns an error in a given HTTP response.
func handleError(w http.ResponseWriter, err error, c *appengine.Context) {
	if c != nil {
		(*c).Errorf("Error: %v", err)
	}
	http.Error(w, err.Error(), http.StatusInternalServerError)
}
