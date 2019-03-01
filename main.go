package main

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"text/template"
	"time"

	"github.com/gorilla/sessions"
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

type UserSession struct {
	SessionKey string
	UserID     string
	CreatedAt  time.Time
}

type Link struct {
	Location string
}

type clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

var store *sessions.CookieStore

func main() {
	if err := seedData(); err != nil {
		log.Fatal(err)
	}

	// TODO: Add a random store key and don't store in source code
	store = sessions.NewCookieStore([]byte("SESSION_KEY"))

	http.HandleFunc("/login.html", viewHandler)
	http.HandleFunc("/oauth", oauthHandler)
	http.HandleFunc("/oauth/callback", oauthCallbackHandler)
	http.HandleFunc("/links.html", handlerWithAuth(linksHandler))
	http.HandleFunc("/links", handlerWithAuth(saveHandler))
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

func saveUserSession(u *User, ctx context.Context, cl clock) (*UserSession, error) {
	s := &UserSession{UserID: u.ID, CreatedAt: cl.Now()}
	k := datastore.NewIncompleteKey(ctx, "UserSession", nil)

	if _, err := datastore.Put(ctx, k, s); err != nil {
		return nil, err
	}

	return s, nil
}

func oauthHandler(w http.ResponseWriter, r *http.Request) {
	conf := oauthConfig()

	// TODO: Generate a random state token
	url := conf.AuthCodeURL("state", oauth2.AccessTypeOffline)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
	return
}

func storeUserSession(w http.ResponseWriter, r *http.Request, u *User) {
	sess, err := store.Get(r, "user")
	if err != nil {
		log.Fatal(err)
	}
	sess.Values["user_id"] = u.ID
	if err = sess.Save(r, w); err != nil {
		log.Fatal(err)
	}
}

func handlerWithAuth(fn func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, err := store.Get(r, "user")
		if err != nil {
			log.Fatal(err)
		}
		if s.Values["user_id"] == nil {
			http.Redirect(w, r, "/login.html", http.StatusTemporaryRedirect)
			return
		}
		fn(w, r)
	}
}

func viewHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, r.URL.Path[1:])
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

	user, err := authorizeUserFromJSON(body, ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	// TODO: Don't save the sessions in database
	c := systemClock{}
	saveUserSession(user, ctx, c)

	storeUserSession(w, r, user)

	http.Redirect(w, r, "/index.html", http.StatusTemporaryRedirect)
	return
}

func linksHandler(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)

	var links []Link

	q := datastore.NewQuery("Link")

	if _, err := q.GetAll(ctx, &links); err != nil {
		log.Fatal(err)
	}

	t, err := template.ParseFiles("links.html")
	if err != nil {
		log.Fatal(err)
	}
	t.Execute(w, links)
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

	http.Redirect(w, r, "/links.html", http.StatusTemporaryRedirect)
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
