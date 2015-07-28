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
	Id        string    `json:"id"`
	Username  string    `json:"username"`
	FirstName string    `json:"firstName"`
	LastName  string    `json:"lastName"`
	Private   bool      `json:"private"`
	Created   time.Time `json:"created"`
	Modified  time.Time `json:"modified"`
}

func (appUser *AppUser) IsValid() bool {
	return appUser.Id != "" && appUser.Username != ""
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
	username := getRequestVar(r, "username", c)
	username = strings.ToLower(username)

	userID, err := getUserID(username, c)
	if err != nil {
		c.Infof("Could not find user with username '%v': %v", username, err.Error())
		http.Error(w, "User not found.", http.StatusNotFound)
		return
	}

	currentUser := user.Current(c)

	if !canDeleteAppUser(userID, currentUser) {
		c.Errorf("%v cannot delete user %v.", currentUser.Email, userID)
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
	username := getRequestVar(r, "username", c)
	username = strings.ToLower(username)

	c.Infof("Getting user %v", username)
	userID, err := getUserID(username, c)
	if err != nil {
		c.Infof("Could not find user with username '%v': %v", username, err.Error())
		http.Error(w, "User not found.", http.StatusNotFound)
		return
	}

	c.Infof("User ID is %v", userID)

	appUser, err := fetchAppUser(userID, c)
	if err != nil {
		c.Infof("Could not fetch user '%v': %v", userID, err.Error())
		http.Error(w, "User not found.", http.StatusNotFound)
		return
	} else if appUser.Private {
		c.Infof("App user '%v' is private. Access Denied.", userID)
		http.Error(w, "This user is private.", http.StatusForbidden)
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

	if !appUser.IsValid() {
		c.Infof("Invalid user object in request: %v", appUser)
		http.Error(w, "Invalid user object!", http.StatusBadRequest)
		return
	}

	u := user.Current(c)
	if appUser.Id != u.Email {
		c.Infof("User '%v' attempted to create a user with a different ID: '%v'. Denied.", u.Email, appUser.Id)
		http.Error(w, "You are not authorized to create a user with a different ID than your own!", http.StatusForbidden)
		return
	}

	// Check if user exists
	existingUser, err := fetchAppUser(u.Email, c)
	if existingUser != nil {
		c.Infof("User with ID '%v' already exists. Cannot create a new user with that ID.", u.Email)
		http.Error(w, fmt.Sprintf("Sorry, the username '%v' is already taken!", u.Email), http.StatusConflict)
		return
	}

	appUser.Username = strings.ToLower(appUser.Username)
	appUser.Created = time.Now()
	appUser.Modified = appUser.Created

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

	if !appUser.IsValid() {
		c.Infof("Invalid user object in request: %v", appUser)
		http.Error(w, "Invalid user object!", http.StatusBadRequest)
		return
	}

	u := user.Current(c)
	// Verify the user already exists
	existingUser, err := fetchAppUser(u.Email, c)
	if existingUser != nil {
		appUser.Username = strings.ToLower(appUser.Username)
		appUser.Created = existingUser.Created
		appUser.Modified = time.Now()
	} else {
		c.Errorf("Cannot update a user '%v'. No such user.", appUser.Id)
		http.Error(w, fmt.Sprintf("User %v does not exist, cannot update.", appUser.Id), http.StatusBadRequest)
	}

	if appUser.Id != u.Email {
		c.Infof("User %v attempted to modify user %v. Denied.", u.Email, appUser.Id)
		http.Error(w, "Not authorized to change another user!", http.StatusForbidden)
		return
	}

	c.Infof("Updated user %v...", appUser.Id)

	_, err = saveAppUser(appUser, c)
	if err != nil {
		handleError(w, err, &c)
	}

	c.Infof("Updated user %v.", appUser.Id)

	resp := UserResponse{true, *appUser}
	sendJsonResponse(w, resp)
}

func fetchAppUser(userID string, c appengine.Context) (*AppUser, error) {
	appUser := new(AppUser)
	userKey := getUserDSKey(userID, c)
	err := datastore.Get(c, userKey, appUser)
	if err != nil {
		return nil, err
	} else {
		return appUser, nil
	}
}

func deleteAppUser(userID string, c appengine.Context) error {
	userKey := getUserDSKey(userID, c)
	err := datastore.Delete(c, userKey)
	return err
}

func saveAppUser(appUser *AppUser, c appengine.Context) (*datastore.Key, error) {
	userKey := getUserDSKey(appUser.Id, c)
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

func getUserDSKey(userID string, c appengine.Context) *datastore.Key {
	userKeyID := createUserKeyID(userID, c)
	userKey := datastore.NewKey(c, USER_KIND, userKeyID, 0, nil)
	return userKey
}

func createUserKeyID(userID string, c appengine.Context) string {
	if userID == "" {
		c.Warningf("Creating an AppUser entity key with no userID!")
	}

	return "user:" + userID
}

func canDeleteAppUser(userID string, currentUser *user.User) bool {
	return userID == currentUser.Email
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
