package api

import (
	"appengine"
	"appengine/datastore"

	"github.com/reedperry/gogram/storage"

	"errors"
	"fmt"
	"net/http"
	"time"
)

const POST_KIND = "post"

type Post struct {
	UserId   string    `json:"user"`
	Id       string    `json:"id"`
	EventId  string    `json:"event"`
	Image    string    `json:"image"`
	Text     string    `json:"text"`
	Created  time.Time `json:"posted"`
	Modified time.Time `json:"modified"`
}

type ViewPost struct {
	Username string    `json:"username"`
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

func (post *Post) createFileName() string {
	return fmt.Sprintf("%v/%v", post.UserId, post.Id)
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
	currentUser, err := getRequestUser(r)
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
		c.Infof("Invalid Post request object.")
		http.Error(w, "Invalid post data.", http.StatusBadRequest)
		return
	}

	id, err := NewUId(c)
	if err != nil {
		c.Errorf("Failed to generate post ID: %v", err)
		http.Error(w, "Failed to create a post.", http.StatusInternalServerError)
		return
	}

	_, err = fetchPost(id, c)
	if err != datastore.ErrNoSuchEntity {
		c.Errorf("Duplicate post ID generated! Aborting. Error: %v", err)
		http.Error(w, "Failed to create a post, please try again.", http.StatusInternalServerError)
		return
	}

	now := time.Now()
	post := &Post{
		UserId:   currentUser.Email,
		Id:       id,
		EventId:  reqPost.EventId,
		Image:    "",
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
	postId := getRequestVar(r, "id", c)

	currentUser, err := getRequestUser(r)
	if err != nil {
		c.Errorf("Must be signed in to create post: %v\n", err)
		http.Error(w, "Not signed in.", http.StatusForbidden)
		return
	}

	post, err := fetchPost(postId, c)
	if err != nil {
		c.Errorf("Cannot attach image - no post found with ID %v.", postId)
		http.Error(w, "Failed to post image.", http.StatusNotFound)
		return
	}

	postUser, err := fetchAppUser(post.UserId, c)
	if err != nil {
		c.Errorf("Could not find AppUser with ID %v: %v", post.UserId, err)
		http.Error(w, "Failed to post image.", http.StatusNotFound)
		return
	} else if postUser.Id != currentUser.Email {
		c.Errorf("User with ID %v cannot attach an image to a post by user ID %v", currentUser.Email, postUser.Id)
		http.Error(w, "Cannot post for a different user.", http.StatusForbidden)
		return
	}

	if post.Image != "" {
		c.Errorf("Cannot attach image - Post %v by user %v already has an associated image.", postUser.Id, postId)
		http.Error(w, "Cannot overwrite the image in a post.", http.StatusForbidden)
		return
	}

	// TODO Validate size, anything else about request data if necessary...

	filename := post.createFileName()
	obj, err := storage.Create(filename, r)
	if err != nil {
		c.Errorf("Failed to store image for user %v: %v", post.UserId, err)
		http.Error(w, "An error occurred while attempting to save the file.", http.StatusInternalServerError)
		return
	}

	c.Infof("Stored file %v for user %v.", filename, post.UserId)

	post.Image = storage.ObjectLink(obj)
	post.Modified = time.Now()

	_, err = savePost(post, c)
	if err != nil {
		c.Errorf("Failed to store updated Post (ID=%v) by user %v: %v", post.Id, postUser.Id, err)
		http.Error(w, "Failed to update the post.", http.StatusInternalServerError)
		return
	}

	sendJsonResponse(w, post)
}

func GetPost(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	postId := getRequestVar(r, "id", c)

	post, err := fetchPost(postId, c)
	if err != nil {
		c.Infof("Could not fetch post %v: %v", postId, err.Error())
		http.Error(w, "Post not found.", http.StatusNotFound)
		return
	}

	postUser, err := fetchAppUser(post.UserId, c)
	if err != nil {
		c.Infof("User %v who created post %v could not be found: %v", post.UserId, postId, err)
		http.Error(w, "Post not found.", http.StatusNotFound)
		return
	}

	currentUser, err := getRequestUser(r)

	if postUser.Private && (currentUser == nil || currentUser.Email != postUser.Id) {
		http.Error(w, "This user is private.", http.StatusForbidden)
		return
	}

	viewPost := &ViewPost{
		Username: postUser.Username,
		Id:       post.Id,
		EventId:  post.EventId,
		Image:    post.Image,
		Text:     post.Text,
		Created:  post.Created,
		Modified: post.Modified,
	}

	sendJsonResponse(w, viewPost)
}

func UpdatePost(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	postId := getRequestVar(r, "id", c)

	post, err := fetchPost(postId, c)
	if err != nil {
		c.Errorf("Cannot update - post ID %v not found.", postId)
		http.Error(w, "Post not found.", http.StatusNotFound)
		return
	}

	postUser, err := fetchAppUser(post.UserId, c)
	if err != nil {
		c.Infof("User %v who created post %v could not be found: %v", post.UserId, postId, err)
		http.Error(w, "Post not found.", http.StatusNotFound)
		return
	}

	currentUser, err := getRequestUser(r)
	if currentUser.Email != postUser.Id {
		http.Error(w, "You can only update your own posts.", http.StatusForbidden)
		return
	}

	updatedPost := new(Post)
	err = readEntity(r, updatedPost)
	if err != nil {
		c.Errorf("Failed to read post data from request: %v", err)
		http.Error(w, "Invalid post data in request.", http.StatusBadRequest)
		return
	}

	if !updatedPost.IsValidRequest() {
		c.Infof("Invalid Post request object.")
		http.Error(w, "Invalid post data.", http.StatusBadRequest)
		return
	}

	if updatedPost.EventId != post.EventId {
		c.Infof("Cannot move post from event %v to event %v!", post.EventId, updatedPost.EventId)
		http.Error(w, "Cannot move this post to a different event.", http.StatusBadRequest)
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
	postId := getRequestVar(r, "id", c)

	post, err := fetchPost(postId, c)
	if err != nil {
		c.Errorf("Cannot delete - post ID %v not found.", postId)
		http.Error(w, "Post not found.", http.StatusNotFound)
		return
	}

	postUser, err := fetchAppUser(post.UserId, c)
	if err != nil {
		c.Infof("User %v who created post %v could not be found: %v", post.UserId, postId, err)
		http.Error(w, "Post not found.", http.StatusNotFound)
		return
	}

	currentUser, err := getRequestUser(r)
	if currentUser.Email != postUser.Id {
		http.Error(w, "You can only update your own posts.", http.StatusForbidden)
		return
	}

	err = deletePost(postId, c)
	if err != nil {
		c.Errorf("Failed to delete post %v from user %v.", postId, postUser.Id)
		http.Error(w, "Failed to delete post.", http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("%v/%v", post.UserId, postId)
	err = storage.Delete(filename, r)
	if err != nil {
		// TODO Add a retry to task queue if we can?
		c.Errorf("Failed to delete file %v for user %v: %v", filename, post.UserId, err)
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

func fetchPost(postID string, c appengine.Context) (*Post, error) {
	post := new(Post)
	postKey := getPostDSKey(postID, c)

	err := datastore.Get(c, postKey, post)
	if err != nil {
		return nil, err
	} else {
		return post, nil
	}
}

func savePost(post *Post, c appengine.Context) (*datastore.Key, error) {
	postKey := getPostDSKey(post.Id, c)
	key, err := datastore.Put(c, postKey, post)
	if err != nil {
		return nil, err
	} else {
		return key, nil
	}
}

func deletePost(postId string, c appengine.Context) error {
	postKey := getPostDSKey(postId, c)

	err := datastore.Delete(c, postKey)
	return err
}

func getPostDSKey(postID string, c appengine.Context) *datastore.Key {
	postKey := datastore.NewKey(c, POST_KIND, postID, 0, nil)
	return postKey
}
