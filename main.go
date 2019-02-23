package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
)

type User struct {
	ID      string
	Email   string
	Picture string
}

type Link struct {
	Location string
}

func main() {
	http.HandleFunc("/login.html", loginHandler)
	http.HandleFunc("/oauth", oauthHandler)
	http.HandleFunc("/oauth/callback", oauthCallbackHandler)
	http.HandleFunc("/links", saveHandler)
	http.HandleFunc("/", redirectHandler)
	appengine.Main()
}

func oauthConfig() *oauth2.Config {
	jsonFile, err := os.Open("client_credentials.json")
	if err != nil {
		log.Fatal(err)
	}
	defer jsonFile.Close()

	json, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		log.Fatal(err)
	}

	config, err := google.ConfigFromJSON(
		json,
		"https://www.googleapis.com/auth/userinfo.email",
	)
	if err != nil {
		log.Fatal(err)
	}
	return config
}

func authorizeUserFromJSON(data []byte, c context.Context) (*User, error) {
	u := new(User)
	if err := json.Unmarshal(data, u); err != nil {
		log.Fatal(err)
	}

	k := datastore.NewKey(c, "User", u.ID, 0, nil)
	user := new(User)
	if err := datastore.Get(c, k, user); err != nil {
		return nil, errors.New("Failed to authorize user")
	}
	user.ID = k.StringID()
	return user, nil
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, r.URL.Path[1:])
}

func oauthHandler(w http.ResponseWriter, r *http.Request) {
	conf := oauthConfig()

	// TODO: Generate a random state token
	url := conf.AuthCodeURL("state", oauth2.AccessTypeOffline)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
	return
}

func oauthCallbackHandler(w http.ResponseWriter, r *http.Request) {
	conf := oauthConfig()

	// TODO: Validate state token
	q := r.URL.Query()
	code := q.Get("code")
	tok, err := conf.Exchange(context.TODO(), code)
	if err != nil {
		log.Fatal(err)
	}

	client := conf.Client(context.Background(), tok)
	res, err := client.Get("https://www.googleapis.com/userinfo/v2/me")
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	ctx := appengine.NewContext(r)

	_, err = authorizeUserFromJSON(body, ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	// TODO: Redirect to link index
	fmt.Fprint(w, "Welcome!")
	return
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
