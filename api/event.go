package api

import (
	"appengine"
	"appengine/datastore"
	"appengine/user"

	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const EVENT_KIND = "event"
const DEFAULT_FEED_ORDER = "-Created"

// Maximum time between an even start and end
const MAX_EVENT_LENGTH = time.Hour * 168 // 1 week
// How far in the future an event can be scheduled to start
const MAX_START_FUTURE = time.Hour * 672 // 4 weeks

type Event struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"desc"`
	Start       time.Time `json:"start"`
	End         time.Time `json:"end"`
	Private     bool      `json:"private"`
	Creator     string    `json:"creator"`
	Created     time.Time `json:"created"`
	Modified    time.Time `json:"modified"`
}

type EventView struct {
	Name        string     `json:"name"`
	Description string     `json:"desc"`
	Start       time.Time  `json:"start"`
	End         time.Time  `json:"end"`
	Creator     string     `json:"creator"`
	Created     time.Time  `json:"created"`
	Modified    time.Time  `json:"modified"`
	Posts       []PostView `json:"posts"`
}

type CreateEventResponse struct {
	Ok bool   `json:"ok"`
	ID string `json:"id"`
}

type EventInfoResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"desc"`
	Start       time.Time `json:"start"`
	End         time.Time `json:"end"`
	IsActive    bool      `json:"isActive"`
}

type ErrPrivateEvent struct{}

func (e *ErrPrivateEvent) Error() string {
	return "This event is private."
}

// IsValid determines if an event object contains all required parts to
// be stored in the database, and that their values are valid.
func (event *Event) IsValid() bool {
	if event.ID == "" || event.Name == "" || event.Description == "" ||
		event.Creator == "" || event.Created.IsZero() {

		return false
	}

	if !event.HasValidDuration() {
		return false
	}

	return true
}

// IsValidRequest determines if an event object sent in a user request
// is contains all required parts, and their values are valid.
func (event *Event) IsValidRequest() bool {
	if event.Name == "" || event.Description == "" {
		return false
	}

	if !event.HasValidDuration() {
		return false
	}

	return true
}

func (event *Event) HasValidDuration() bool {
	if event.Start.IsZero() || event.End.IsZero() {
		return false
	}

	if event.Start.After(event.End) || event.End.Before(time.Now()) {
		return false
	}

	if event.End.Sub(event.Start) > MAX_EVENT_LENGTH {
		return false
	}

	if event.Start.Sub(time.Now()) > MAX_START_FUTURE {
		return false
	}

	return true
}

func (event *Event) IsActive() bool {
	if !event.HasValidDuration() {
		return false
	}

	now := time.Now()
	return event.End.After(now) && event.Start.Before(now)
}

func (event *Event) AuthorizeView(c appengine.Context) error {
	if !event.Private {
		return nil
	}

	u := user.Current(c)
	if u == nil {
		c.Infof("No user signed in - Cannot access private event.")
		return new(ErrPrivateEvent)
	} else {
		appUser, err := FetchAppUser(u.ID, c)
		if err != nil {
			c.Infof("Cannot find user with ID %v - Cannot access private event.", u.ID)
			return new(ErrPrivateEvent)
		}

		cv := event.userCanView(appUser)
		if !cv {
			c.Infof("User %v is not authorized to view private event %v.", appUser.ID, event.ID)
			return new(ErrPrivateEvent)
		}
	}

	return nil
}

// TODO Not implemented yet - always denies view for private events
func (event *Event) userCanView(appUser *AppUser) bool {
	return false
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

	eID, err := NewUID(c)
	if err != nil {
		c.Errorf("Failed to generate event ID: %v", err)
		http.Error(w, "Failed to create a new event.", http.StatusInternalServerError)
		return
	}

	_, err = FetchEvent(eID, c)
	if err != datastore.ErrNoSuchEntity {
		c.Errorf("Duplicate event ID generated! Aborting. Error: %v", err)
		http.Error(w, "Failed to create a event, please try again.", http.StatusInternalServerError)
		return
	}

	event.ID = eID
	event.Creator = existingUser.ID
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

	resp := CreateEventResponse{true, event.ID}
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
	eventID := GetRequestVar(r, "id", c)
	event, err := FetchEvent(eventID, c)
	if err != nil {
		c.Errorf("Failed to fetch event with ID %v: %v", eventID, err)
		http.NotFound(w, r)
		return
	}

	err = event.AuthorizeView(c)
	if err != nil {
		http.Error(w, "This event is private. You are not authorized to view it.", http.StatusForbidden)
		return
	}

	resp := EventInfoResponse{
		ID:          event.ID,
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

	eventID := GetRequestVar(r, "id", c)
	event, err := FetchEvent(eventID, c)
	if err != nil {
		c.Errorf("Failed to fetch event with ID %v: %v", eventID, err)
		http.NotFound(w, r)
		return
	}

	if event.Creator != existingUser.ID {
		c.Errorf("User %v tried to update event created by %v - denied.", existingUser.ID, event.Creator)
		http.Error(w, "You are not authorized to updated this event.", http.StatusForbidden)
		return
	}

	updated := new(Event)
	err = readEntity(r, updated)
	if err != nil {
		c.Errorf("Failed to read event data from request: %v", err)
		http.Error(w, "Could not read event from request.", http.StatusBadRequest)
		return
	}

	if !updated.IsValidRequest() {
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

	if !event.IsValid() {
		c.Errorf("Event failed validation, aborting update.")
		http.Error(w, "Failed to update the event.", http.StatusInternalServerError)
		return
	}

	_, err = storeEvent(event, c)
	if err != nil {
		c.Errorf("Failed to store updated Event with ID %v: %v", event.ID, err)
		http.Error(w, "Failed to update the event.", http.StatusInternalServerError)
		return
	}

	resp := EventInfoResponse{
		ID:          event.ID,
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

func FetchEvent(eventID string, c appengine.Context) (*Event, error) {
	eventKey, err := getEventDSKey(eventID, c)
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

	var orderBy string = DEFAULT_FEED_ORDER
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

func storeEvent(event *Event, c appengine.Context) (*datastore.Key, error) {
	eventKey, err := getEventDSKey(event.ID, c)
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
