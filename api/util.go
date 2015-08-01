package api

import (
	"appengine"
	"appengine/datastore"

	"github.com/gorilla/mux"
	"github.com/nu7hatch/gouuid"

	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const EPOCH = 1420070400000 // Jan 1 2015 midnight GMT

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

func GetRequestVar(r *http.Request, varName string, c appengine.Context) string {
	vars := mux.Vars(r)
	value, ok := vars[varName]

	if !ok {
		c.Warningf("No var '%v' present in request URL.", varName)
	}

	return value
}

func NewUId(c appengine.Context) (string, error) {
	var id uint64
	millis := time.Now().UnixNano() / 1000000
	millis -= EPOCH

	id = uint64(millis) << 16

	random, err := uuid.NewV4()
	if err != nil {
		return "", err
	}

	randomStr := strings.Replace(random.String(), "-", "", -1)
	hexStr := randomStr[23:27]
	val, err := strconv.ParseInt(hexStr, 16, 32)
	if err != nil {
		return "", nil
	}

	num := uint16(val)
	id |= (uint64(num) % 65536)

	return strconv.FormatUint(id, 16), nil
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
