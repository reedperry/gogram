package api

import (
	"appengine"
	"appengine/datastore"
	"appengine/user"

	"errors"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"
)

const POST_KIND = "post"
const NO_IMAGE = "__none__"

type Post struct {
	UserId   string    `json:"user"`
	Id       string    `json:"id"`
	EventId  string    `json:"event"`
	Image    string    `json:"image"`
	Text     string    `json:"text"`
	Created  time.Time `json:"posted"`
	Modified time.Time `json:"modified"`
}

func (post *Post) IsValid() bool {
	if post.UserId == "" || post.Id == "" || post.EventId == "" || post.Created.IsZero() {
		return false
	}

	return true
}

func (post *Post) IsValidRequest() bool {
	if post.EventId == "" {
		return false
	}

	return true
}

type CreatePostResponse struct {
	Ok bool   `json:"ok"`
	Id string `json:"id"`
}

type Comment struct {
	UserId string    `json:"userId"`
	PostId string    `json:"postId"`
	Text   string    `json:"text"`
	Date   time.Time `json:"date"`
}

func CreatePost(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	u, err := getRequestUser(r)
	if err != nil {
		c.Errorf("Must be signed in to create post: %v\n", err)
		http.Error(w, "Not signed in.", http.StatusForbidden)
		return
	}

	reqPost := new(Post)
	if err := readEntity(r, reqPost); err != nil {
		handleError(w, err, &c)
	}

	if !reqPost.IsValidRequest() {
		c.Errorf("Invalid Post request object.")
		http.Error(w, "Invalid post data.", http.StatusBadRequest)
		return
	}

	id := strconv.Itoa(time.Now().Nanosecond())
	now := time.Now()
	post := &Post{
		UserId:   u.Email,
		Id:       id,
		EventId:  reqPost.EventId,
		Image:    NO_IMAGE,
		Text:     reqPost.Text,
		Created:  now,
		Modified: now,
	}

	if !post.IsValid() {
		c.Errorf("Invalid Post object, cannot store.")
		http.Error(w, "Failed to create post.", http.StatusInternalServerError)
		return
	}

	_, err = savePost(post, c)
	if err != nil {
		handleError(w, err, &c)
	}

	resp := CreatePostResponse{true, id}
	w.WriteHeader(http.StatusCreated)
	sendJsonResponse(w, resp)
}

// AttachImage stores and associates an image file with a Post.
// This function should be called immediately after a successfull call to CreatePost.
func AttachImage(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	username := getRequestVar(r, "username", c)
	postId := getRequestVar(r, "id", c)

	u, err := getRequestUser(r)
	if err != nil {
		c.Errorf("Must be signed in to create post: %v\n", err)
		http.Error(w, "Not signed in.", http.StatusForbidden)
		return
	}

	postUser, err := fetchAppUserByName(username, c)
	if err != nil {
		c.Errorf("Could not find AppUser for username %v: %v", username, err)
		http.Error(w, "Failed to post image.", http.StatusInternalServerError)
		return
	} else if postUser.Id != u.Email {
		c.Errorf("User with ID %v cannot attach an image to a post by user with ID %v", u.Email, postUser.Id)
		http.Error(w, "Cannot post for a different user.", http.StatusForbidden)
		return
	}

	post, err := fetchPost(postUser.Id, postId, c)
	if err != nil {
		c.Errorf("Cannot attach image - no post found for user %v with post ID %v.", postUser.Id, postId)
		http.Error(w, "Failed to post image.", http.StatusInternalServerError)
		return
	}

	if post.Image != NO_IMAGE {
		c.Errorf("Cannot attach image - Post %v by user %v already has an associated image.", postUser.Id, postId)
		http.Error(w, "Cannot overwrite the image in a post.", http.StatusForbidden)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		c.Errorf("Failed to read request body: %v\n", err)
		http.Error(w, "Failed to read request body!", http.StatusInternalServerError)
		return
	} else {
		c.Infof("Request body: %s", body)
	}

	c.Infof("Recieved post for user %v, request content length is %v", u.Email, r.ContentLength)

	c.Infof("Attachment of images not implemented yet...")

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

func UpdatePost(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	username := getRequestVar(r, "username", c)
	postId := getRequestVar(r, "id", c)

	postUser, err := fetchAppUserByName(username, c)
	if err != nil {
		http.Error(w, "Post not found.", http.StatusNotFound)
		return
	}

	currentUser := user.Current(c)
	if currentUser.Email != postUser.Id {
		http.Error(w, "You can only update your own posts.", http.StatusForbidden)
		return
	}

	post, err := fetchPost(postUser.Id, postId, c)
	if err != nil {
		c.Errorf("Cannot update - post ID %v not found for user %v.", post.Id, postUser.Id)
		http.Error(w, "Post not found.", http.StatusNotFound)
		return
	}

	updatedPost := new(Post)
	err = readEntity(r, updatedPost)
	if err != nil {
		c.Errorf("Failed to read post data from request: %v", err)
		http.Error(w, "Invalid post data in request.", http.StatusBadRequest)
		return
	}

	// Update post data
	post.Text = updatedPost.Text
	post.Modified = time.Now()

	_, err = savePost(post, c)
	if err != nil {
		c.Errorf("Failed to store updated Post (ID=%v) by user %v: %v", post.Id, postUser.Id, err)
		http.Error(w, "Failed to update the post.", http.StatusInternalServerError)
		return
	}

	sendJsonResponse(w, post)
}

func DeletePost(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	username := getRequestVar(r, "username", c)
	postId := getRequestVar(r, "id", c)

	postUser, err := fetchAppUserByName(username, c)
	if err != nil {
		http.Error(w, "Post not found.", http.StatusNotFound)
		return
	}

	currentUser := user.Current(c)
	if currentUser.Email != postUser.Id {
		http.Error(w, "You can only delete your own posts.", http.StatusForbidden)
		return
	}

	err = deletePost(postId, postUser.Id, c)
	if err != nil {
		c.Errorf("Failed to delete post %v from user %v.", postId, postUser.Id)
		http.Error(w, "Failed to delete post.", http.StatusInternalServerError)
		return
	}

	c.Infof("Deleted post %v from user %v.", postId, postUser.Id)

	resp := OkResponse{true}
	sendJsonResponse(w, resp)
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

func savePost(post *Post, c appengine.Context) (*datastore.Key, error) {
	postKey, err := getPostDSKey(post.UserId, post.Id, c)
	key, err := datastore.Put(c, postKey, post)
	if err != nil {
		return nil, err
	} else {
		return key, nil
	}
}

func deletePost(postId, userId string, c appengine.Context) error {
	postKey, err := getPostDSKey(userId, postId, c)
	if err != nil {
		c.Errorf("Failed to create Post Key ID")
		return err
	}

	err = datastore.Delete(c, postKey)
	return err
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
