package gogram

import (
	"github.com/gorilla/mux"
	"github.com/reedperry/gogram/api"
)

func Router() *mux.Router {
	r := mux.NewRouter()
	r.StrictSlash(true)

	r.HandleFunc("/u", api.CreateUser).Methods("POST")
	r.HandleFunc("/u/{username}", api.GetUser).Methods("GET")
	r.HandleFunc("/u/{username}", api.UpdateUser).Methods("PUT")
	r.HandleFunc("/u/{username}", api.DeleteUser).Methods("DELETE")

	r.HandleFunc("/p", api.CreatePost).Methods("POST")
	r.HandleFunc("/p/{id}/attach", api.AttachImage).Methods("POST")
	r.HandleFunc("/p/{id}", api.GetPost).Methods("GET")
	r.HandleFunc("/p/{id}", api.DeletePost).Methods("DELETE")
	r.HandleFunc("/p/{id}", api.UpdatePost).Methods("PUT")

	r.HandleFunc("/e", api.CreateEvent).Methods("POST")
	r.HandleFunc("/e/{id}", api.GetEvent).Methods("GET")
	r.HandleFunc("/e/{id}", api.DeleteEvent).Methods("DELETE")
	r.HandleFunc("/e/{id}", api.UpdateEvent).Methods("PUT")

	r.HandleFunc("/", ServeApp).Methods("GET")
	return r
}
