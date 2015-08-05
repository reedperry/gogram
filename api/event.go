package api

import (
	"appengine"
	"appengine/datastore"

	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const EVENT_KIND = "event"
const DEFAULT_ORDER = "-Created"

type Event struct {
	Id          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"desc"`
	Start       time.Time `json:"start"`
	End         time.Time `json:"expiration"`
	Private     bool      `json:"private"`
	Creator     string    `json:"creator"`
	Created     time.Time `json:"created"`
	Modified    time.Time `json:"modified"`
}

type EventView struct {
	Name        string     `json:"name"`
	Description string     `json:"desc"`
	Start       time.Time  `json:"start"`
	End         time.Time  `json:"expiration"`
	Creator     string     `json:"creator"`
	Created     time.Time  `json:"created"`
	Modified    time.Time  `json:"modified"`
	Posts       []PostView `json:"posts"`
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
	End         time.Time `json:"expiration"`
	IsActive    bool      `json:"isActive"`
}

func (event *Event) IsValid() bool {
	if event.Id == "" || event.Name == "" ||
		event.Start.IsZero() || event.End.IsZero() ||
		event.Creator == "" || event.Created.IsZero() {

		return false
	}

	if event.Start.After(event.End) || event.End.Before(time.Now()) {
		return false
	}

	return true
}

// TODO Put date range limitations on event start/end times
func (event *Event) IsValidRequest() bool {
	if event.Id == "" || event.Name == "" || event.Start.IsZero() || event.End.IsZero() {
		return false
	}

	if event.Start.After(event.End) || event.End.Before(time.Now()) {
		return false
	}

	return true
}

func (event *Event) IsActive() bool {
	if event.Start.IsZero() || event.End.IsZero() {
		return false
	}

	now := time.Now()
	return event.End.After(now) && event.Start.Before(now)
}

func CreateEvent(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	u, err := getRequestUser(r)
	if err != nil {
		c.Errorf("Must be signed in to create event: %v\n", err)
		http.Error(w, "Not signed in.", http.StatusForbidden)
		return
	}

	existingUser, err := FetchAppUser(u.ID, c)
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

	if !event.IsValidRequest() {
		c.Infof("Invalid event request object.")
		http.Error(w, "Invalid event data.", http.StatusBadRequest)
		return
	}

	eId, err := NewUId(c)
	if err != nil {
		c.Errorf("Failed to generate event ID: %v", err)
		http.Error(w, "Failed to create a new event.", http.StatusInternalServerError)
		return
	}

	_, err = FetchEvent(eId, c)
	if err != datastore.ErrNoSuchEntity {
		c.Errorf("Duplicate event ID generated! Aborting. Error: %v", err)
		http.Error(w, "Failed to create a event, please try again.", http.StatusInternalServerError)
		return
	}

	event.Id = eId
	event.Creator = existingUser.Id
	now := time.Now()
	event.Created = now
	event.Modified = now

	// If Start is in the past, align it with current time.
	if event.Start.Before(now) {
		event.Start = now
	}

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

func EventsFeed(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	page := GetRequestVar(r, "page", c)
	order := GetRequestVar(r, "order", c)

	events, err := fetchEventFeed(page, order, c)
	if err != nil {
		http.Error(w, "Failed to fetch event feed.", http.StatusInternalServerError)
	}

	sendJsonResponse(w, events)
}

func GetEvent(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	eventId := GetRequestVar(r, "id", c)
	event, err := FetchEvent(eventId, c)
	if err != nil {
		c.Errorf("Failed to fetch event with ID %v: %v", eventId, err)
		http.NotFound(w, r)
		return
	}

	resp := EventInfoResponse{
		Id:          event.Id,
		Name:        event.Name,
		Description: event.Description,
		Start:       event.Start,
		End:         event.End,
		IsActive:    event.IsActive(),
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

	existingUser, err := FetchAppUser(u.ID, c)
	if err != nil {
		c.Errorf("Not a registered user, cannot update an Event: %v", err)
		http.Error(w, "Must register to create or update events.", http.StatusForbidden)
		return
	}

	eventId := GetRequestVar(r, "id", c)
	event, err := FetchEvent(eventId, c)
	if err != nil {
		c.Errorf("Failed to fetch event with ID %v: %v", eventId, err)
		http.NotFound(w, r)
		return
	}

	if event.Creator != existingUser.Id {
		c.Errorf("User %v tried to update event created by %v - denied.", existingUser.Id, event.Creator)
		http.Error(w, "You are not authorized to updated this event.", http.StatusForbidden)
		return
	}

	updated := new(Event)
	err = readEntity(r, updated)
	if err != nil {
		c.Errorf("Failed to read event data from request: %v", err)
		http.Error(w, "Invalid event data in request.", http.StatusBadRequest)
		return
	}

	if !event.IsValidRequest() {
		c.Infof("Invalid event request object.")
		http.Error(w, "Invalid event data.", http.StatusBadRequest)
		return
	}

	// Copy fields that can be modified
	event.Name = updated.Name
	event.Description = updated.Description
	event.Private = updated.Private
	// TODO Do we allow extending events that have expired?
	event.End = updated.End

	now := time.Now()
	// Only allow modifying the start time if it is still in the future.
	if event.Start != updated.Start && event.Start.After(now) {
		event.Start = updated.Start
	}

	// If Start is in the past, align it with current time.
	if event.Start.Before(now) {
		event.Start = now
	}

	event.Modified = time.Now()

	_, err = storeEvent(event, c)
	if err != nil {
		c.Errorf("Failed to store updated Event with ID %v: %v", event.Id, err)
		http.Error(w, "Failed to update the event.", http.StatusInternalServerError)
		return
	}

	resp := EventInfoResponse{
		Id:          event.Id,
		Name:        event.Name,
		Description: event.Description,
		Start:       event.Start,
		End:         event.End,
		IsActive:    event.IsActive(),
	}
	sendJsonResponse(w, resp)
}

// DeleteEvent permanently removes an Event, along with all Posts associated with the Event.
// Not sure whether this will really be used during regular operation...
func DeleteEvent(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Not implemented."))
}

func storeEvent(event *Event, c appengine.Context) (*datastore.Key, error) {
	eventKey, err := getEventDSKey(event.Id, c)
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

func FetchEvent(eventId string, c appengine.Context) (*Event, error) {
	eventKey, err := getEventDSKey(eventId, c)
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

func fetchEventFeed(page, order string, c appengine.Context) (*[]Event, error) {
	var pageNum int = 0
	if page != "" {
		// Ignore error and default to page 0
		pageNum, _ = strconv.Atoi(page)
	}

	var orderBy string = DEFAULT_ORDER
	if validFeedOrder(order) {
		orderBy = order
	}

	events := make([]Event, 0, 20)

	q := datastore.NewQuery(EVENT_KIND).
		Order(orderBy).
		Limit(20).
		Offset(20 * pageNum)

	_, err := q.GetAll(c, &events)
	if err != nil {
		c.Errorf("Failed to get event feed: %v", err)
		return nil, err
	}

	return &events, nil
}

func validFeedOrder(order string) bool {
	if order == "" {
		return false
	}

	if strings.HasPrefix(order, "-") {
		order = order[1:]
	}

	if order == "Created" || order == "End" {
		return true
	}

	return false
}

func getEventDSKey(eventID string, c appengine.Context) (*datastore.Key, error) {
	if eventID == "" {
		return nil, errors.New("No eventID provided.")
	}

	eventKeyID := createEventKeyID(eventID, c)
	eventKey := datastore.NewKey(c, EVENT_KIND, eventKeyID, 0, nil)
	return eventKey, nil
}

func createEventKeyID(eventID string, c appengine.Context) string {
	if eventID == "" {
		c.Errorf("Creating an event entity key with no eventID!")
	}

	return "event:" + eventID
}
