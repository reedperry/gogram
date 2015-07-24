package gogram

import (
	"appengine"
	"appengine/datastore"

	"github.com/nu7hatch/gouuid"

	"errors"
	"net/http"
	"strings"
	"time"
)

const EVENT_KIND = "event"

type Event struct {
	Id          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"desc"`
	Start       time.Time `json:"start"`
	Expiration  time.Time `json:"expiration"`
	Posts       []string  `json:"posts"`
	Private     bool      `json:"private"`
	Creator     string    `json:"creator"`
	Created     time.Time `json:"created"`
}

type CreateEventResponse struct {
	Ok bool   `json:"ok"`
	Id string `json:"id"`
}

func (event *Event) IsValid() bool {
	if event.Id == "" || event.Name == "" ||
		event.Start.IsZero() || event.Expiration.IsZero() ||
		event.Creator == "" || event.Created.IsZero() {

		return false
	}

	if event.Start.After(event.Expiration) || event.Expiration.Before(time.Now()) {
		return false
	}

	return true
}

func (event *Event) createDSKey(c appengine.Context) (*datastore.Key, error) {
	if event.Id == "" {
		return nil, errors.New("Event is missing id")
	}

	eventKey := datastore.NewKey(c, EVENT_KIND, event.Id, 0, nil)
	return eventKey, nil
}

func CreateEvent(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	u, err := getRequestUser(r)
	if err != nil {
		c.Errorf("Must be signed in to create event: %v\n", err)
		http.Error(w, "Not signed in.", http.StatusForbidden)
		return
	}

	existingUser, err := fetchAppUser(u.Email, c)
	if err != nil {
		c.Errorf("Not a registered user, cannot create an Event: %v", err)
		http.Error(w, "Must register to create an event.", http.StatusForbidden)
		return
	}

	event := new(Event)
	if err := readEntity(r, event); err != nil {
		c.Errorf("Failed to read event new data from request body: %v", err)
		http.Error(w, "Invalid event creation request.", http.StatusBadRequest)
		return
	}

	eId, err := uuid.NewV4()
	if err != nil {
		c.Errorf("Failed to generate event UUID: %v", err)
		http.Error(w, "Failed to create a new event.", http.StatusInternalServerError)
		return
	}

	event.Id = strings.Replace(eId.String(), "-", "", -1)
	event.Creator = existingUser.Username
	event.Created = time.Now()

	if !event.IsValid() {
		c.Errorf("Event failed validation, aborting save.")
		http.Error(w, "Failed to create a new event.", http.StatusInternalServerError)
		return
	}

	if _, err = storeEvent(event, c); err != nil {
		c.Errorf("Failed to store Event: %v", err)
		http.Error(w, "Failed to create a new event.", http.StatusInternalServerError)
		return
	}

	resp := CreateEventResponse{true, event.Id}
	sendJsonResponse(w, resp)
}

func GetEvent(w http.ResponseWriter, r *http.Request) {
}

func UpdateEvent(w http.ResponseWriter, r *http.Request) {
}

func DeleteEvent(w http.ResponseWriter, r *http.Request) {
}

func storeEvent(event *Event, c appengine.Context) (*datastore.Key, error) {
	eventKey, err := event.createDSKey(c)
	if err != nil {
		c.Errorf("Failed to create event entity key: %v\n", err)
		return nil, err
	}

	if key, err := datastore.Put(c, eventKey, event); err != nil {
		c.Errorf("Failed to store event entity: %v\n", err)
		return nil, err
	} else {
		return key, nil
	}
}
