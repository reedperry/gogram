package gogram

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

type Post struct {
	User     string    `json:"user"`
	Id       string    `json:"id"`
	Image    string    `json:"image"`
	Posted   time.Time `json:"posted"`
	Likes    []string  `json:"likes"`
	Comments []string  `json:"comments"`
}

type CreatePostResponse struct {
	Ok bool   `json:"ok"`
	Id string `json:"id"`
}

type Comment struct {
	UserId string    `json:"userId"`
	Text   string    `json:"text"`
	Date   time.Time `json:"date"`
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
	postKey, err := getPostDSKey(post.User, post.Id, c)
	key, err := datastore.Put(c, postKey, post)
	if err != nil {
		return nil, err
	} else {
		return key, nil
	}
}

func CreatePost(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	u, err := getRequestUser(r)
	if err != nil {
		c.Errorf("No user to create post: %v\n", err)
		http.Error(w, "No user signed in.", http.StatusForbidden)
		return
	}

	id := strconv.Itoa(time.Now().Nanosecond())
	var likes, comments []string
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
