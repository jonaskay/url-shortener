package main

import (
	"net/http"
	"os"

	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
)

type Link struct {
	Location string
}

func main() {
	http.HandleFunc("/links", saveHandler)
	http.HandleFunc("/", redirectHandler)
	appengine.Main()
}

func saveHandler(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)

	id := r.FormValue("id")
	location := r.FormValue("location")

	k := datastore.NewKey(ctx, "Link", id, 0, nil)
	l := &Link{Location: location}

	if _, err := datastore.Put(ctx, k, l); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	return
}

func redirectHandler(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)

	slug := r.URL.Path[len("/"):]
	if slug == "" {
		http.Redirect(
			w,
			r,
			os.Getenv("DEFAULT_REDIRECT_LOCATION"),
			http.StatusMovedPermanently,
		)
	}

	k := datastore.NewKey(ctx, "Link", slug, 0, nil)
	l := new(Link)

	if err := datastore.Get(ctx, k, l); err != nil {
		http.Error(
			w,
			http.StatusText(http.StatusNotFound),
			http.StatusNotFound,
		)
		return
	}
	http.Redirect(w, r, l.Location, http.StatusMovedPermanently)
	return
}
