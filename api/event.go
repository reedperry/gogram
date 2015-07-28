package api

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
	Private     bool      `json:"private"`
	Creator     string    `json:"creator"`
	Created     time.Time `json:"created"`
	Modified    time.Time `json:"modified"`
}

type CreateEventResponse struct {
	Ok bool   `json:"ok"`
	Id string `json:"id"`
}

type EventInfoResponse struct {
	Id          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"desc"`
	Start       time.Time `json:"start"`
	Expiration  time.Time `json:"expiration"`
	IsActive    bool      `json:"isActive"`
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
	event.Creator = existingUser.Id
	now := time.Now()
	event.Created = now
	event.Modified = now

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
	w.WriteHeader(http.StatusCreated)
	sendJsonResponse(w, resp)
}

func GetEvent(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	eventId := getRequestVar(r, "id", c)
	event, err := fetchEvent(eventId, c)
	if err != nil {
		c.Errorf("Failed to fetch event with ID %v: %v", eventId, err)
		http.Error(w, "Event not found.", http.StatusNotFound)
		return
	}

	now := time.Now()
	isActive := event.Expiration.After(now) && event.Start.Before(now)

	resp := EventInfoResponse{
		Id:          event.Id,
		Name:        event.Name,
		Description: event.Description,
		Start:       event.Start,
		Expiration:  event.Expiration,
		IsActive:    isActive,
	}
	sendJsonResponse(w, resp)
}

func UpdateEvent(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	u, err := getRequestUser(r)
	if err != nil {
		c.Errorf("Must be signed in to create event: %v\n", err)
		http.Error(w, "Not signed in.", http.StatusForbidden)
		return
	}

	existingUser, err := fetchAppUser(u.Email, c)
	if err != nil {
		c.Errorf("Not a registered user, cannot update an Event: %v", err)
		http.Error(w, "Must register to create or update events.", http.StatusForbidden)
		return
	}

	eventId := getRequestVar(r, "id", c)
	event, err := fetchEvent(eventId, c)
	if err != nil {
		c.Errorf("Failed to fetch event with ID %v: %v", eventId, err)
		http.Error(w, "Event not found.", http.StatusNotFound)
		return
	}

	if event.Creator != existingUser.Id {
		c.Errorf("User %v tried to update event created by %v - denied.", existingUser.Id, event.Creator)
		http.Error(w, "You are not authorized to updated this event.", http.StatusNotFound)
		return
	}

	updated := new(Event)
	err = readEntity(r, updated)
	if err != nil {
		c.Errorf("Failed to read event data from request: %v", err)
		http.Error(w, "Invalid event data in request.", http.StatusBadRequest)
		return
	}

	// Copy fields that can be modified
	event.Name = updated.Name
	event.Description = updated.Description
	event.Private = updated.Private
	event.Start = updated.Start
	event.Expiration = updated.Expiration

	event.Modified = time.Now()

	_, err = storeEvent(event, c)
	if err != nil {
		c.Errorf("Failed to store updated Event with ID %v: %v", event.Id, err)
		http.Error(w, "Failed to update the event.", http.StatusInternalServerError)
		return
	}

	now := time.Now()
	isActive := event.Expiration.After(now) && event.Start.Before(now)

	resp := EventInfoResponse{
		Id:          event.Id,
		Name:        event.Name,
		Description: event.Description,
		Start:       event.Start,
		Expiration:  event.Expiration,
		IsActive:    isActive,
	}
	sendJsonResponse(w, resp)
}

// DeleteEvent permanently removes an Event, along with all Posts associated with the Event.
// Not sure whether this will really be used during regular operation...
func DeleteEvent(w http.ResponseWriter, r *http.Request) {
}

func storeEvent(event *Event, c appengine.Context) (*datastore.Key, error) {
	eventKey, err := createDSKey(event.Id, c)
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

func fetchEvent(eventId string, c appengine.Context) (*Event, error) {
	eventKey, err := createDSKey(eventId, c)
	if err != nil {
		c.Errorf("Cannot fetch Event: %v", err)
		return nil, err
	}

	event := new(Event)
	err = datastore.Get(c, eventKey, event)
	if err != nil {
		return nil, err
	}

	return event, nil
}

func createDSKey(eventId string, c appengine.Context) (*datastore.Key, error) {
	if eventId == "" {
		return nil, errors.New("Cannot create key with empty ID.")
	}

	eventKey := datastore.NewKey(c, EVENT_KIND, eventId, 0, nil)
	return eventKey, nil
}
