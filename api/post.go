package api

import (
	"appengine"
	"appengine/datastore"
	"appengine/taskqueue"

	"github.com/reedperry/gogram/imgstore"

	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const POST_KIND = "post"

type Post struct {
	UserID   string    `json:"user"`
	ID       string    `json:"id"`
	EventID  string    `json:"event"`
	Image    string    `json:"image"`
	Text     string    `json:"text"`
	Created  time.Time `json:"posted"`
	Modified time.Time `json:"modified"`
}

type PostView struct {
	Username string    `json:"username"`
	ID       string    `json:"id"`
	EventID  string    `json:"event"`
	Image    string    `json:"image"`
	Text     string    `json:"text"`
	Created  time.Time `json:"posted"`
	Modified time.Time `json:"modified"`
}

func (post *Post) IsValid() bool {
	if post.UserID == "" || post.ID == "" || post.EventID == "" || post.Created.IsZero() {
		return false
	}

	return true
}

func (post *Post) IsValidRequest() bool {
	if post.EventID == "" {
		return false
	}

	return true
}

func (post *Post) createFileName() string {
	return fmt.Sprintf("%v/%v", post.UserID, post.ID)
}

type CreatePostResponse struct {
	Ok bool   `json:"ok"`
	ID string `json:"id"`
}

type Comment struct {
	UserID string    `json:"userId"`
	PostID string    `json:"postId"`
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

	// Validate that the event ID matches an existing, active event
	event, err := FetchEvent(reqPost.EventID, c)
	if err != nil {
		c.Infof("Could not find event %v referenced by post: %v", reqPost.EventID, err)
		http.Error(w, "Post does not match an existing event.", http.StatusBadRequest)
		return
	}

	if !event.IsActive() {
		c.Infof("Cannot post to inactive event %v.", reqPost.EventID)
		http.Error(w, "This event is not currently active.", http.StatusForbidden)
		return
	}

	id, err := NewUID(c)
	if err != nil {
		c.Errorf("Failed to generate post ID: %v", err)
		http.Error(w, "Failed to create a post.", http.StatusInternalServerError)
		return
	}

	_, err = FetchPost(id, c)
	if err != datastore.ErrNoSuchEntity {
		c.Errorf("Duplicate post ID generated! Aborting. Error: %v", err)
		http.Error(w, "Failed to create a post, please try again.", http.StatusInternalServerError)
		return
	}

	now := time.Now()
	post := &Post{
		UserID:   currentUser.ID,
		ID:       id,
		EventID:  reqPost.EventID,
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
	postID := GetRequestVar(r, "id", c)

	currentUser, err := getRequestUser(r)
	if err != nil {
		c.Errorf("Must be signed in to create post: %v\n", err)
		http.Error(w, "Not signed in.", http.StatusForbidden)
		return
	}

	post, err := FetchPost(postID, c)
	if err != nil {
		c.Errorf("Cannot attach image - no post found with ID %v.", postID)
		http.NotFound(w, r)
		return
	}

	postUser, err := FetchAppUser(post.UserID, c)
	if err != nil {
		c.Errorf("Could not find AppUser with ID %v: %v", post.UserID, err)
		http.Error(w, "Failed to post image: user not found.", http.StatusInternalServerError)
		return
	} else if postUser.ID != currentUser.ID {
		c.Errorf("User with ID %v cannot attach an image to a post by user ID %v", currentUser.ID, postUser.ID)
		http.Error(w, "Cannot post for a different user.", http.StatusForbidden)
		return
	}

	if post.Image != "" {
		c.Errorf("Cannot attach image - Post %v by user %v already has an image attached.", postUser.ID, postID)
		http.Error(w, "Cannot overwrite the image in a post.", http.StatusForbidden)
		return
	}

	// TODO Validate size, anything else about request data if necessary...

	filename := post.createFileName()
	obj, err := imgstore.Create(filename, r)
	if err != nil {
		c.Errorf("Failed to store image for user %v: %v", post.UserID, err)
		http.Error(w, "An error occurred while attempting to save the file.", http.StatusInternalServerError)
		return
	}

	c.Infof("Stored file %v for user %v.", filename, post.UserID)

	if err = queueProcessing(filename, c); err != nil {
		c.Errorf("Failed to add file %v for post %v to image processing queue.", filename, post.ID)
	}

	post.Image = imgstore.ObjectLink(obj)
	post.Modified = time.Now()

	_, err = savePost(post, c)
	if err != nil {
		c.Errorf("Failed to store updated Post (ID=%v) by user %v: %v", post.ID, postUser.ID, err)
		http.Error(w, "Failed to update the post.", http.StatusInternalServerError)
		return
	}

	sendJsonResponse(w, post)
}

func GetPost(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	postID := GetRequestVar(r, "id", c)

	post, err := FetchPost(postID, c)
	if err != nil {
		c.Infof("Could not fetch post %v: %v", postID, err.Error())
		http.NotFound(w, r)
		return
	}

	postUser, err := FetchAppUser(post.UserID, c)
	if err != nil {
		c.Infof("User %v who created post %v could not be found: %v", post.UserID, postID, err)
		http.NotFound(w, r)
		return
	}

	postView := &PostView{
		Username: postUser.Username,
		ID:       post.ID,
		EventID:  post.EventID,
		Image:    post.Image,
		Text:     post.Text,
		Created:  post.Created,
		Modified: post.Modified,
	}

	sendJsonResponse(w, postView)
}

func UpdatePost(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	postID := GetRequestVar(r, "id", c)

	post, err := FetchPost(postID, c)
	if err != nil {
		c.Errorf("Cannot update - post ID %v not found.", postID)
		http.NotFound(w, r)
		return
	}

	postUser, err := FetchAppUser(post.UserID, c)
	if err != nil {
		c.Infof("User %v who created post %v could not be found: %v", post.UserID, postID, err)
		http.NotFound(w, r)
		return
	}

	currentUser, err := getRequestUser(r)
	if currentUser.ID != postUser.ID {
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

	if updatedPost.EventID != post.EventID {
		c.Infof("Cannot move post from event %v to event %v!", post.EventID, updatedPost.EventID)
		http.Error(w, "Cannot move this post to a different event.", http.StatusBadRequest)
		return
	}

	// Validate that the event ID matches an existing, active event
	event, err := FetchEvent(updatedPost.EventID, c)
	if err != nil {
		c.Infof("Could not find event %v referenced by post: %v", updatedPost.EventID, err)
		http.Error(w, "Post does not match an existing event.", http.StatusBadRequest)
		return
	}

	if !event.IsActive() {
		c.Infof("Cannot update a post for inactive event %v.", updatedPost.EventID)
		http.Error(w, "This event is not currently active.", http.StatusForbidden)
		return
	}

	// Update post data
	post.Text = updatedPost.Text
	post.Modified = time.Now()

	_, err = savePost(post, c)
	if err != nil {
		c.Errorf("Failed to store updated Post (ID=%v) by user %v: %v", post.ID, postUser.ID, err)
		http.Error(w, "Failed to update the post.", http.StatusInternalServerError)
		return
	}

	sendJsonResponse(w, post)
}

func DeletePost(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	postID := GetRequestVar(r, "id", c)

	post, err := FetchPost(postID, c)
	if err != nil {
		c.Errorf("Cannot delete - post ID %v not found.", postID)
		http.NotFound(w, r)
		return
	}

	postUser, err := FetchAppUser(post.UserID, c)
	if err != nil {
		c.Infof("User %v who created post %v could not be found: %v", post.UserID, postID, err)
		http.NotFound(w, r)
		return
	}

	currentUser, err := getRequestUser(r)
	if currentUser.ID != postUser.ID {
		http.Error(w, "You can only update your own posts.", http.StatusForbidden)
		return
	}

	err = deletePost(postID, c)
	if err != nil {
		c.Errorf("Failed to delete post %v from user %v.", postID, postUser.ID)
		http.Error(w, "Failed to delete post.", http.StatusInternalServerError)
		return
	}

	filename := post.createFileName()
	err = imgstore.Delete(filename, r)
	if err != nil {
		// TODO Add a retry to task queue if we can?
		c.Errorf("Failed to delete file %v for user %v: %v", filename, post.UserID, err)
	}

	c.Infof("Deleted post %v from user %v.", postID, postUser.ID)

	resp := OkResponse{true}
	sendJsonResponse(w, resp)
}

func queueProcessing(filename string, c appengine.Context) error {
	t := taskqueue.NewPOSTTask("/", url.Values{
		"filename": {filename},
	})

	_, err := taskqueue.Add(c, t, "image-processor")

	return err
}

func FetchPost(postID string, c appengine.Context) (*Post, error) {
	post := new(Post)
	postKey, err := getPostDSKey(postID, c)
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

// TODO Need to control offset/limit/sort
func FetchUserPosts(userID string, c appengine.Context) (*[]Post, error) {
	q := datastore.NewQuery(POST_KIND).
		Filter("UserID =", userID).
		Limit(20).
		Order("-Created")

	posts := make([]Post, 0, 20)

	_, err := q.GetAll(c, &posts)
	if err != nil {
		c.Errorf("Failed to get posts for user %v: %v", userID, err)
		return nil, err
	}

	return &posts, nil
}

// TODO Need to control offset/limit/sort
func FetchEventPosts(eventID string, c appengine.Context) (*[]Post, error) {
	q := datastore.NewQuery(POST_KIND).
		Filter("EventID =", eventID).
		Limit(20).
		Order("-Created")

	posts := make([]Post, 0, 20)

	_, err := q.GetAll(c, &posts)
	if err != nil {
		c.Errorf("Failed to get posts for event %v: %v", eventID, err)
		return nil, err
	}

	return &posts, nil
}

func savePost(post *Post, c appengine.Context) (*datastore.Key, error) {
	postKey, err := getPostDSKey(post.ID, c)
	if err != nil {
		return nil, err
	}

	key, err := datastore.Put(c, postKey, post)
	if err != nil {
		return nil, err
	} else {
		return key, nil
	}
}

func deletePost(postID string, c appengine.Context) error {
	postKey, err := getPostDSKey(postID, c)
	if err != nil {
		return err
	}

	err = datastore.Delete(c, postKey)
	return err
}

func getPostDSKey(postID string, c appengine.Context) (*datastore.Key, error) {
	if postID == "" {
		return nil, errors.New("No postID provided.")
	}

	postKeyID := createPostKeyID(postID, c)
	postKey := datastore.NewKey(c, POST_KIND, postKeyID, 0, nil)
	return postKey, nil
}

func createPostKeyID(postID string, c appengine.Context) string {
	if postID == "" {
		c.Errorf("Creating a post entity key with no postID!")
	}

	return "post:" + postID
}
