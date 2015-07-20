package gogram

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/context"
	"github.com/gorilla/mux"

	"appengine"
	"appengine/datastore"
	"appengine/user"
)

type ctxKey int

const userCtxKey ctxKey = 0

const USER_KIND = "appUser"
const POST_KIND = "post"

func init() {
	http.Handle("/", Authorize(Router()))
}

func Authorize(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := appengine.NewContext(r)
		_, err := authorize(r, c)
		if err != nil {
			if r.URL.Path == "/" {
				loginURL, _ := user.LoginURL(c, "/")
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				fmt.Fprintf(w, `You are not signed in! Sign in <a href="%s">here</a>.`, loginURL)
			} else {
				c.Infof("User authorization failed: %v", err)
				http.Error(w, "Not Authorized!", http.StatusForbidden)
			}

			return
		}

		h.ServeHTTP(w, r)
	})
}

func Router() *mux.Router {
	r := mux.NewRouter()
	r.StrictSlash(true)
	r.HandleFunc("/user", CreateUser).Methods("POST")
	r.HandleFunc("/user/{username}", GetUser).Methods("GET")
	r.HandleFunc("/user/{username}", UpdateUser).Methods("PUT")
	r.HandleFunc("/user/{username}", DeleteUser).Methods("DELETE")
	r.HandleFunc("/post", CreatePost).Methods("POST")
	r.HandleFunc("/post/{username}/{id}", GetPost).Methods("GET")
	//r.HandleFunc("/post/{username}/{id}", DeletePost).Methods("DELETE")
	//r.HandleFunc("/post/{username}/{id}", UpdatePost).Methods("PUT")
	r.HandleFunc("/", ServeApp).Methods("GET")
	return r
}

func ServeApp(w http.ResponseWriter, r *http.Request) {
	content, err := ioutil.ReadFile("index.html")
	if err != nil {
		fmt.Fprint(w, "index.html not found!")
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, string(content))
}

func DeleteUser(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	username := getRequestVar(r, "username", c)

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

func fetchAppUserByName(username string, c appengine.Context) (*AppUser, error) {
	q := datastore.NewQuery(USER_KIND).
		Filter("Username =", username)

	for r := q.Run(c); ; {
		var u AppUser
		_, err := r.Next(&u)
		if err == datastore.Done {
			return nil, errors.New("No user with username " + username)
		}
		if err != nil {
			c.Warningf("Failed to fetch AppUser with username '%v'\n", username)
			return nil, err
		}

		return &u, nil
	}
}

func fetchPost(userID, postID string, c appengine.Context) (*Post, error) {
	post := new(Post)
	postKey, err := getPostDSKey(userID, postID, c)
	if err != nil {
		return nil, err
	}
	err = datastore.Get(c, postKey, post)
	if err != nil {
		return nil, err
	} else {
		return post, nil
	}
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

func savePost(post *Post, c appengine.Context) (*datastore.Key, error) {
	postKey, err := getPostDSKey(post.User, post.Id, c)
	key, err := datastore.Put(c, postKey, post)
	if err != nil {
		return nil, err
	} else {
		return key, nil
	}
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

	appUser.Created = time.Now()
	appUser.Modified = appUser.Created

	c.Infof("Creating user %v...", appUser.Id)

	_, err = saveAppUser(appUser, c)
	if err != nil {
		handleError(w, err, &c)
	}

	c.Infof("Created user %v.", appUser.Id)

	resp := UserResponse{true, *appUser}
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

func CreatePost(w http.ResponseWriter, r *http.Request) {
	var likes, comments []string
	c := appengine.NewContext(r)
	u, err := getRequestUser(r)
	if err != nil {
		c.Errorf("No user to create post: %v\n", err)
		http.Error(w, "No user signed in.", http.StatusForbidden)
		return
	}

	id := strconv.Itoa(time.Now().Nanosecond())
	post := &Post{u.Email, id, id, time.Now(), likes, comments}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		c.Errorf("Failed to read request body: %v\n", err)
		http.Error(w, "Failed to read request body!", http.StatusInternalServerError)
		return
	} else {
		c.Infof("Request body: %s", body)
		post.Image = string(body)
	}

	c.Infof("Recieved post for user %v, request content length is %v", u.Email, r.ContentLength)
	c.Infof("Post contents: %v", post)

	_, err = savePost(post, c)
	if err != nil {
		handleError(w, err, &c)
	}

	resp := CreatePostResponse{true, id}
	sendJsonResponse(w, resp)
}

func GetPost(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	username := getRequestVar(r, "username", c)
	postId := getRequestVar(r, "id", c)

	postUser, err := fetchAppUserByName(username, c)
	if err != nil {
		http.Error(w, "Post not found.", http.StatusNotFound)
		return
	}

	currentUser := user.Current(c)
	if postUser.Private && currentUser.Email != postUser.Id {
		http.Error(w, "This user is private.", http.StatusForbidden)
		return
	}

	post, err := fetchPost(postUser.Id, postId, c)
	if err != nil {
		c.Infof("Could not fetch post %v from user ID %v. %v", postId, postUser.Id, err.Error())
		http.Error(w, "Post not found.", http.StatusNotFound)
		return
	}

	sendJsonResponse(w, post)
}

func getPostDSKey(userID, postID string, c appengine.Context) (*datastore.Key, error) {
	postKeyID, err := createPostKeyID(userID, postID)
	if err != nil {
		c.Errorf("Failed creating Post Key ID: %v", err)
		return nil, err
	}
	postKey := datastore.NewKey(c, POST_KIND, postKeyID, 0, nil)
	return postKey, nil
}

func createPostKeyID(userID, postID string) (string, error) {
	if userID == "" || postID == "" {
		return "", errors.New("Missing userID or postID, cannot create Post Key ID")
	}

	return userID + "-" + postID, nil
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

func (appUser *AppUser) IsValid() bool {
	return appUser.Id != "" && appUser.Username != ""
}

func canDeleteAppUser(userID string, currentUser *user.User) bool {
	return userID == currentUser.Email
}

// Authorize verifies that the user making the request should be allowed
// to continue. If the user is not authorized, an error is returned with
// a nil user value.
// When the user is signed in, this function stores the user
func authorize(r *http.Request, c appengine.Context) (*user.User, error) {
	u := user.Current(c)
	if u == nil {
		return nil, errors.New("Authorization failed. User not logged in.")
	}

	context.Set(r, userCtxKey, u)

	return u, nil
}

func getRequestUser(r *http.Request) (*user.User, error) {
	val, ok := context.GetOk(r, userCtxKey)
	if !ok {
		return nil, errors.New("No user signed in.")
	}

	u, ok := val.(*user.User)
	if !ok {
		return nil, errors.New("No user signed in.")
	}

	return u, nil
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

type Post struct {
	User     string    `json:"user"`
	Id       string    `json:"id"`
	Image    string    `json:"image"`
	Posted   time.Time `json:"posted"`
	Likes    []string  `json:"likes"`
	Comments []string  `json:"comments"`
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

type HasCustomDatastoreKey interface {
	DSKey(appengine.Context) (*datastore.Key, error)
	DSKeyID(appengine.Context) (string, error)
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

type Comment struct {
	UserId string    `json:"userId"`
	Text   string    `json:"text"`
	Date   time.Time `json:"date"`
}

type UserResponse struct {
	Ok   bool    `json:"ok"`
	Data AppUser `json:"data"`
}

type CreatePostResponse struct {
	Ok bool   `json:"ok"`
	Id string `json:"id"`
}

type OkResponse struct {
	Ok bool `json:"ok"`
}
