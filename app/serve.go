package app

import (
	"appengine"

	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"

	"github.com/reedperry/gogram/api"
	"github.com/reedperry/gogram/middleware"
)

func init() {
	http.Handle("/", middleware.Authorize(Router()))
}

func ServeEventFeed(w http.ResponseWriter, r *http.Request) {
	content, err := ioutil.ReadFile("index.html")
	if err != nil {
		fmt.Fprint(w, "index.html not found!")
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, string(content))
}

func ServeEvent(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	id := api.GetRequestVar(r, "id", c)
	if id == "" {
		http.Error(w, "Missing event ID.", http.StatusBadRequest)
	}

	t, err := template.ParseFiles("templates/event.html")
	if err != nil {
		c.Errorf("Failed to parse event template: %v", err)
		http.Error(w, "Failed to load event.", http.StatusInternalServerError)
		return
	}

	event, err := api.FetchEvent(id, c)
	if err != nil {
		http.Error(w, "Failed to fetch event.", http.StatusInternalServerError)
		return
	}

	posts, err := api.FetchPosts(event.Id, c)
	if err != nil {
		http.Error(w, "Failed to fetch posts for event.", http.StatusInternalServerError)
		return
	}

	eventPosts := make([]api.PostView, 0, 20)
	for _, post := range *posts {

		// Fill in current username for found posts
		appUser, err := api.FetchAppUser(post.UserId, c)
		var username = "[deleted]"
		if err != nil {
			c.Infof("No user found for post %v", post.Id)
		} else {
			username = appUser.Username
		}

		eventPosts = append(eventPosts, api.PostView{
			Username: username,
			Id:       post.Id,
			EventId:  post.EventId,
			Image:    post.Image,
			Text:     post.Text,
			Created:  post.Created,
			Modified: post.Modified,
		})
	}

	// Fill in current username for event creator
	appUser, err := api.FetchAppUser(event.Creator, c)
	var creator = "[deleted]"
	if err != nil {
		c.Infof("No user found for event %v", event.Id)
	} else {
		creator = appUser.Username
	}

	data := &api.EventView{
		Name:        event.Name,
		Description: event.Description,
		Start:       event.Start,
		End:         event.End,
		Creator:     creator,
		Created:     event.Created,
		Modified:    event.Modified,
		Posts:       eventPosts,
	}

	t.Execute(w, data)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
}

func ServePost(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	id := api.GetRequestVar(r, "id", c)
	if id == "" {
		http.Error(w, "Missing post ID.", http.StatusBadRequest)
	}

	t, err := template.ParseFiles("templates/post.html")
	if err != nil {
		c.Errorf("Failed to parse post template: %v", err)
		http.Error(w, "Failed to load post.", http.StatusInternalServerError)
		return
	}

	post, err := api.FetchPost(id, c)
	if err != nil {
		http.Error(w, "Failed to fetch post.", http.StatusInternalServerError)
		return
	}

	appUser, err := api.FetchAppUser(post.UserId, c)
	var username = "[deleted]"
	if err != nil {
		c.Infof("No user found for post %v", post.Id)
	} else {
		username = appUser.Username
	}

	postView := &api.PostView{
		Username: username,
		Id:       post.Id,
		EventId:  post.EventId,
		Image:    post.Image,
		Text:     post.Text,
		Created:  post.Created,
		Modified: post.Modified,
	}

	t.Execute(w, postView)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
}
