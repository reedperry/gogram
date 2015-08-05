package api

import (
	"appengine"
	"appengine/datastore"
	"appengine/user"

	"github.com/gorilla/context"

	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const USER_KIND = "appUser"
const UserCtxKey ctxKey = 0

type UserResponse struct {
	Ok   bool    `json:"ok"`
	Data AppUser `json:"data"`
}

type AppUser struct {
	Email     string    `json:"-"`
	Id        string    `json:"id"`
	Username  string    `json:"username"`
	FirstName string    `json:"firstName"`
	LastName  string    `json:"lastName"`
	Created   time.Time `json:"created"`
	Modified  time.Time `json:"modified"`
}

func (appUser *AppUser) IsValid() bool {
	return appUser.Id != "" && appUser.Username != "" &&
		!appUser.Created.IsZero() && !appUser.Modified.IsZero()
}

func (appUser *AppUser) IsValidRequest() bool {
	return appUser.Username != ""
}

func (appUser *AppUser) DSKey(c appengine.Context) (*datastore.Key, error) {
	userKeyID, err := appUser.DSKeyID(c)
	if err != nil {
		return nil, err
	}
	userKey := datastore.NewKey(c, USER_KIND, userKeyID, 0, nil)
	return userKey, nil
}

func (appUser *AppUser) DSKeyID(c appengine.Context) (string, error) {
	if appUser.Id == "" {
		c.Warningf("Attempted to create an AppUser entity key with no Id!")
		return "", errors.New("AppUser has no Id!")
	}

	return "user:" + appUser.Id, nil
}

func DeleteUser(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	username := GetRequestVar(r, "username", c)
	username = strings.ToLower(username)

	userID, err := getUserID(username, c)
	if err != nil {
		c.Infof("Could not find user with username '%v': %v", username, err.Error())
		http.NotFound(w, r)
		return
	}

	currentUser := user.Current(c)

	if !canDeleteAppUser(userID, currentUser) {
		c.Errorf("%v cannot delete user %v.", currentUser.ID, userID)
		http.Error(w, "You cannot delete another user.", http.StatusForbidden)
		return
	}

	c.Infof("Deleting user %v...", userID)

	err = deleteAppUser(userID, c)
	if err != nil {
		c.Errorf("Failed to delete user %v: %v", userID, err)
		http.Error(w, "Failed to delete user.", http.StatusInternalServerError)
		return
	}

	c.Infof("Deleted user %v.", userID)

	resp := OkResponse{true}
	sendJsonResponse(w, resp)
}

func GetUser(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	username := GetRequestVar(r, "username", c)
	username = strings.ToLower(username)

	c.Infof("Getting user %v", username)
	userID, err := getUserID(username, c)
	if err != nil {
		c.Infof("Could not find user with username '%v': %v", username, err.Error())
		http.NotFound(w, r)
		return
	}

	c.Infof("User ID is %v", userID)

	appUser, err := FetchAppUser(userID, c)
	if err != nil {
		c.Infof("Could not fetch user '%v': %v", userID, err.Error())
		http.NotFound(w, r)
		return
	}

	resp := UserResponse{true, *appUser}
	sendJsonResponse(w, resp)
}

func CreateUser(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	appUser := new(AppUser)
	if err := readEntity(r, appUser); err != nil {
		handleError(w, err, &c)
	}

	if !appUser.IsValidRequest() {
		c.Infof("Got an invalid AppUser request: %+v", appUser)
		http.Error(w, "Invalid user data.", http.StatusBadRequest)
		return
	}

	// Copy over data from signed-in user account
	u := user.Current(c)
	appUser.Id = u.ID
	appUser.Email = u.Email

	// Check if a user already exists for this account
	existingUser, err := FetchAppUser(u.ID, c)
	if existingUser != nil {
		c.Infof("User with ID '%v' already exists. Cannot create a new user with that ID.", u.ID)
		http.Error(w, fmt.Sprintf("You already have an account with the username '%v'.", existingUser.Username),
			http.StatusConflict)
		return
	}

	// Verify the username is not already in use
	appUser.Username = strings.ToLower(appUser.Username)
	_, err = fetchAppUserByName(appUser.Username, c)
	if err == nil {
		c.Infof("Username %v is already in use.", appUser.Username)
		http.Error(w, fmt.Sprintf("Sorry, the username '%v' is already taken!", appUser.Username), http.StatusConflict)
		return
	} else {
		if err != datastore.ErrNoSuchEntity {
			c.Errorf("Failed to query for user with existing username %v.\n%v", appUser.Username, err)
			http.Error(w, "An error occurred during registration.", http.StatusInternalServerError)
			return
		}
	}

	appUser.Created = time.Now()
	appUser.Modified = appUser.Created

	if !appUser.IsValid() {
		c.Errorf("Cannot store invalid user object: %+v", appUser)
		http.Error(w, "An error occurred during registration.", http.StatusInternalServerError)
		return
	}

	c.Infof("Creating user %v...", appUser.Id)

	_, err = saveAppUser(appUser, c)
	if err != nil {
		handleError(w, err, &c)
	}

	c.Infof("Created user %v.", appUser.Id)

	resp := UserResponse{true, *appUser}
	w.WriteHeader(http.StatusCreated)
	sendJsonResponse(w, resp)
}

func UpdateUser(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	appUser := new(AppUser)
	if err := readEntity(r, appUser); err != nil {
		handleError(w, err, &c)
	}

	if !appUser.IsValidRequest() {
		c.Infof("Got an invalid AppUser request: %+v", appUser)
		http.Error(w, "Invalid user data.", http.StatusBadRequest)
		return
	}

	u := user.Current(c)
	// Verify the user already exists
	existingUser, err := FetchAppUser(u.ID, c)
	if existingUser != nil {
		appUser.Username = strings.ToLower(appUser.Username)
		appUser.Created = existingUser.Created
		appUser.Modified = time.Now()
	} else {
		c.Errorf("Cannot update a user '%v'. No such user.", appUser.Id)
		http.Error(w, fmt.Sprintf("User %v does not exist, cannot update.", appUser.Id), http.StatusBadRequest)
	}

	if appUser.Id != u.ID {
		c.Infof("User %v attempted to modify user %v. Denied.", u.ID, appUser.Id)
		http.Error(w, "Not authorized to change another user!", http.StatusForbidden)
		return
	}

	if !appUser.IsValid() {
		c.Errorf("Cannot store invalid user object: %+v", appUser)
		http.Error(w, "An error occurred during user update.", http.StatusInternalServerError)
		return
	}

	c.Infof("Updating user %v...", appUser.Id)

	_, err = saveAppUser(appUser, c)
	if err != nil {
		handleError(w, err, &c)
	}

	c.Infof("Updated user %v.", appUser.Id)

	resp := UserResponse{true, *appUser}
	sendJsonResponse(w, resp)
}

func FetchAppUser(userID string, c appengine.Context) (*AppUser, error) {
	appUser := new(AppUser)
	userKey, err := getUserDSKey(userID, c)
	if err != nil {
		return nil, err
	}

	err = datastore.Get(c, userKey, appUser)
	if err != nil {
		return nil, err
	} else {
		return appUser, nil
	}
}

func fetchAppUserByName(username string, c appengine.Context) (*AppUser, error) {
	q := datastore.NewQuery(USER_KIND).
		Filter("Username =", username)

	for r := q.Run(c); ; {
		var u AppUser
		_, err := r.Next(&u)
		if err == datastore.Done {
			return nil, datastore.ErrNoSuchEntity
		}
		if err != nil {
			c.Errorf("Query failed to fetch AppUser with username '%v'\n%v", username, err)
			return nil, err
		}

		return &u, nil
	}
}

func deleteAppUser(userID string, c appengine.Context) error {
	userKey, err := getUserDSKey(userID, c)
	if err != nil {
		return err
	}

	err = datastore.Delete(c, userKey)
	return err
}

func saveAppUser(appUser *AppUser, c appengine.Context) (*datastore.Key, error) {
	userKey, err := getUserDSKey(appUser.Id, c)
	if err != nil {
		return nil, err
	}

	key, err := datastore.Put(c, userKey, appUser)
	if err != nil {
		return nil, err
	} else {
		return key, nil
	}
}

func getUserID(username string, c appengine.Context) (string, error) {
	q := datastore.NewQuery(USER_KIND).
		Filter("Username =", username).
		Project("Id")

	for r := q.Run(c); ; {
		var u AppUser
		_, err := r.Next(&u)
		if err == datastore.Done {
			return "", errors.New("No user with username " + username)
		}
		if err != nil {
			c.Warningf("Failed to look up AppUser with username '%v'\n", username)
			return "", err
		}

		return u.Id, nil
	}
}

func getUserDSKey(userID string, c appengine.Context) (*datastore.Key, error) {
	if userID == "" {
		return nil, errors.New("No userID provided.")
	}

	userKeyID := createUserKeyID(userID, c)
	userKey := datastore.NewKey(c, USER_KIND, userKeyID, 0, nil)
	return userKey, nil
}

func createUserKeyID(userID string, c appengine.Context) string {
	if userID == "" {
		c.Errorf("Creating an appUser entity key with no userID!")
	}

	return "user:" + userID
}

func canDeleteAppUser(userID string, currentUser *user.User) bool {
	return userID == currentUser.ID
}

func getRequestUser(r *http.Request) (*user.User, error) {
	val, ok := context.GetOk(r, UserCtxKey)
	if !ok {
		return nil, errors.New("No user signed in.")
	}

	u, ok := val.(*user.User)
	if !ok {
		return nil, errors.New("No user signed in.")
	}

	return u, nil
}
