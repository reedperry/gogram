package app

import (
	"github.com/gorilla/mux"
	"github.com/reedperry/gogram/api"
)

func Router() *mux.Router {
	r := mux.NewRouter()
	r.StrictSlash(true)

	r.HandleFunc("/a/u", api.CreateUser).Methods("POST")
	r.HandleFunc("/a/u/{username}", api.GetUser).Methods("GET")
	r.HandleFunc("/a/u/{username}", api.UpdateUser).Methods("PUT")
	r.HandleFunc("/a/u/{username}", api.DeleteUser).Methods("DELETE")

	r.HandleFunc("/a/p", api.CreatePost).Methods("POST")
	r.HandleFunc("/a/p/{id}/attach", api.AttachImage).Methods("POST")
	r.HandleFunc("/a/p/{id}", api.GetPost).Methods("GET")
	r.HandleFunc("/a/p/{id}", api.DeletePost).Methods("DELETE")
	r.HandleFunc("/a/p/{id}", api.UpdatePost).Methods("PUT")

	r.HandleFunc("/a/e", api.CreateEvent).Methods("POST")
	r.HandleFunc("/a/e/{id}", api.GetEvent).Methods("GET")
	r.HandleFunc("/a/e/{id}", api.DeleteEvent).Methods("DELETE")
	r.HandleFunc("/a/e/{id}", api.UpdateEvent).Methods("PUT")
	r.HandleFunc("/a/feed/e", api.EventsFeed).Methods("GET")
	r.HandleFunc("/a/feed/e/{page}", api.EventsFeed).Methods("GET")
	r.HandleFunc("/a/feed/e/{order}/{page}", api.EventsFeed).Methods("GET")

	r.HandleFunc("/", ServeEventFeed).Methods("GET")
	r.HandleFunc("/e/{id}", ServeEvent).Methods("GET")
	r.HandleFunc("/p/{id}", ServePost).Methods("GET")
	r.HandleFunc("/u/{username}", ServeUser).Methods("GET")

	return r
}
