package gogram

import (
	"github.com/gorilla/mux"
)

func Router() *mux.Router {
	r := mux.NewRouter()
	r.StrictSlash(true)

	r.HandleFunc("/u", CreateUser).Methods("POST")
	r.HandleFunc("/u/{username}", GetUser).Methods("GET")
	r.HandleFunc("/u/{username}", UpdateUser).Methods("PUT")
	r.HandleFunc("/u/{username}", DeleteUser).Methods("DELETE")

	r.HandleFunc("/p", CreatePost).Methods("POST")
	r.HandleFunc("/p/{username}/{id}", GetPost).Methods("GET")
	r.HandleFunc("/p/{username}/{id}", DeletePost).Methods("DELETE")
	//r.HandleFunc("/p/{username}/{id}", UpdatePost).Methods("PUT")

	r.HandleFunc("/e", CreateEvent).Methods("POST")
	r.HandleFunc("/e/{id}", GetEvent).Methods("GET")
	r.HandleFunc("/e/{id}", DeleteEvent).Methods("DELETE")
	r.HandleFunc("/e/{id}", UpdateEvent).Methods("PUT")

	r.HandleFunc("/", ServeApp).Methods("GET")
	return r
}
