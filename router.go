package gogram

import (
	"github.com/gorilla/mux"
)

func Router() *mux.Router {
	r := mux.NewRouter()
	r.StrictSlash(true)

	r.HandleFunc("/user", CreateUser).Methods("POST")
	r.HandleFunc("/user/{username}", GetUser).Methods("GET")
	r.HandleFunc("/user/{username}", UpdateUser).Methods("PUT")
	r.HandleFunc("/user/{username}", DeleteUser).Methods("DELETE")

	r.HandleFunc("/post", CreatePost).Methods("POST")
	r.HandleFunc("/post/{username}/{id}", GetPost).Methods("GET")
	r.HandleFunc("/post/{username}/{id}", DeletePost).Methods("DELETE")
	//r.HandleFunc("/post/{username}/{id}", UpdatePost).Methods("PUT")

	r.HandleFunc("/", ServeApp).Methods("GET")
	return r
}
