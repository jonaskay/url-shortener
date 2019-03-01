package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/sessions"
	"google.golang.org/appengine"
	"google.golang.org/appengine/aetest"
	"google.golang.org/appengine/datastore"
)

func TestAuthorizeUserFromJSON(t *testing.T) {
	ctx, done, err := aetest.NewContext()
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}
	defer done()

	if err := seedTestDatastore(ctx); err != nil {
		t.Fatalf("Failed to seed datastore: %v", err)
	}

	tt := []struct {
		name string
		data []byte
		err  error
		user *User
	}{
		{
			name: "authorized email",
			data: []byte(`{"id":"42"}`),
			err:  nil,
			user: &User{ID: "42", Email: "jane@example.com"},
		},
		{
			name: "unauthorized email",
			data: []byte(`{"id":"1337"}`),
			err:  errors.New("Failed to authorize user"),
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			u, err := authorizeUserFromJSON(tc.data, ctx)
			if err != nil {
				if err.Error() != tc.err.Error() {
					t.Errorf(
						`Error "%v", want "%v"`,
						err,
						tc.err,
					)
				}
			}
			if tc.user == nil {
				if u != tc.user {
					t.Errorf(
						"User %v, want %v",
						u,
						tc.user,
					)
				}
			} else {
				if u.ID != tc.user.ID {
					t.Errorf(
						"User ID %v, want %v",
						u.ID,
						tc.user.ID,
					)
				}
				if u.Email != tc.user.Email {
					t.Errorf(
						"User email %v, want %v",
						u.Email,
						tc.user.Email,
					)
				}
			}
		})
	}
}

func TestSaveUserSession(t *testing.T) {
	ctx, done, err := aetest.NewContext()
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}
	defer done()

	u := &User{ID: "42"}

	cl := testClock{}
	s, err := saveUserSession(u, ctx, cl)
	if err != nil {
		t.Fatal(err)
	}

	q := datastore.NewQuery("UserSession")
	count, err := q.Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("UserSession count %v, want %v", count, 1)
	}

	if s.UserID != "42" {
		t.Errorf("UserSession UserID %v, want %v", s.UserID, 42)
	}

	if !s.CreatedAt.Equal(cl.Now()) {
		t.Errorf(
			"UserSession CreatedAt %v, want %v",
			s.CreatedAt,
			cl.Now(),
		)
	}
}

func TestOauthRedirect(t *testing.T) {
	os.Setenv("BASE_URL", "http://www.example.com")

	inst, err := aetest.NewInstance(nil)
	if err != nil {
		t.Fatalf("Failed to create instance: %v", err)
	}
	defer inst.Close()

	req, err := inst.NewRequest("GET", "/oauth", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	rec := httptest.NewRecorder()

	oauthHandler(rec, req)
	res := rec.Result()

	if res.StatusCode != http.StatusTemporaryRedirect {
		t.Errorf(
			"Status code %v, want %v",
			res.StatusCode,
			http.StatusTemporaryRedirect,
		)
	}
}

func TestStoreUserSession(t *testing.T) {
	inst, err := aetest.NewInstance(nil)
	if err != nil {
		t.Fatalf("Failed to create instance: %v", err)
	}
	defer inst.Close()

	store = sessions.NewCookieStore([]byte("SESSION_KEY"))

	req, err := inst.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	rec := httptest.NewRecorder()
	u := &User{ID: "42"}

	storeUserSession(rec, req, u)

	sess, err := store.Get(req, "user")
	if err != nil {
		t.Fatal(err)
	}

	id := sess.Values["user_id"]
	want := u.ID
	if id != want {
		t.Errorf("User ID %v, want %v", id, want)
	}
}

func TestAuthentication(t *testing.T) {
	inst, err := aetest.NewInstance(nil)
	if err != nil {
		t.Fatalf("Failed to create instance: %v", err)
	}
	defer inst.Close()

	store = sessions.NewCookieStore([]byte("key"))

	tt := []struct {
		name    string
		handler http.HandlerFunc
		user    *User
		status  int
	}{
		{
			name:    "session with user_id",
			handler: handlerWithAuth(viewHandler),
			user:    &User{ID: "42"},
			status:  200,
		},
		{
			name:    "session without user_id",
			handler: handlerWithAuth(viewHandler),
			status:  307,
		},
		{
			name:    "handler without authentication",
			handler: viewHandler,
			status:  200,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			req, err := inst.NewRequest("GET", "/", nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			rec := httptest.NewRecorder()

			if tc.user != nil {
				storeUserSession(rec, req, tc.user)
			}

			tc.handler(rec, req)
			res := rec.Result()

			if res.StatusCode != tc.status {
				t.Errorf(
					"Status code %v, want %v",
					res.StatusCode,
					tc.status,
				)
			}

			if tc.status == http.StatusTemporaryRedirect {
				loc, err := res.Location()
				if err != nil {
					t.Fatalf(
						"Failed to get location: %v",
						err,
					)
				}

				path := loc.Path
				want := "/login.html"
				if path != want {
					t.Errorf(
						"Location %v, want %v",
						path,
						want,
					)
				}
			}
		})
	}
}

func TestRedirecting(t *testing.T) {
	os.Setenv("DEFAULT_REDIRECT_LOCATION", "http://www.example.com/")

	tt := []struct {
		name   string
		path   string
		status int
		rawurl string
	}{
		{
			name:   "path without slug",
			path:   "/",
			status: 301,
			rawurl: "http://www.example.com/",
		},
		{
			name:   "path with valid slug",
			path:   "/example",
			status: 301,
			rawurl: "http://www.example.com/example",
		},
		{
			name:   "path with unknown slug",
			path:   "/foo",
			status: 404,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			inst, err := aetest.NewInstance(nil)
			if err != nil {
				t.Fatalf("Failed to create instance: %v", err)
			}
			defer inst.Close()

			req, err := inst.NewRequest("GET", tc.path, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			ctx := appengine.NewContext(req)
			if err := seedTestDatastore(ctx); err != nil {
				t.Fatal(err)
			}

			rec := httptest.NewRecorder()

			redirectHandler(rec, req)
			res := rec.Result()

			if res.StatusCode != tc.status {
				t.Errorf(
					"Status code %v, want %v",
					res.StatusCode,
					tc.status,
				)
			}

			if tc.rawurl != "" {
				loc, err := res.Location()
				if err != nil {
					t.Fatalf(
						"Failed to get location: %v",
						err,
					)
				}

				if loc.String() != tc.rawurl {
					t.Errorf(
						"Location URL %v, want %v",
						loc.String(),
						tc.rawurl,
					)
				}
			}
		})
	}
}

func TestSaving(t *testing.T) {
	tt := []struct {
		name   string
		id     string
		status int
		diff   int
	}{
		{
			name:   "link with new id",
			id:     "foobar",
			status: 204,
			diff:   1,
		},
		{
			name:   "link with existing id",
			id:     "example",
			status: 204,
			diff:   0,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			inst, err := aetest.NewInstance(nil)
			if err != nil {
				t.Fatalf("Failed to create instance: %v", err)
			}
			defer inst.Close()

			form := url.Values{}
			form.Add("id", tc.id)
			form.Add("location", "http://www.example.com")

			req, err := inst.NewRequest(
				"POST",
				"/links",
				strings.NewReader(form.Encode()),
			)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			req.Header.Add(
				"Content-Type",
				"application/x-www-form-urlencoded",
			)

			ctx := appengine.NewContext(req)
			if err := seedTestDatastore(ctx); err != nil {
				t.Fatal(err)
			}

			q := datastore.NewQuery("Link")
			c1, err := q.Count(ctx)
			if err != nil {
				t.Fatal(err)
			}

			rec := httptest.NewRecorder()

			saveHandler(rec, req)
			res := rec.Result()

			if res.StatusCode != tc.status {
				t.Errorf(
					"Status code %v, want %v",
					res.StatusCode,
					tc.status,
				)
			}

			c2, err := q.Count(ctx)
			if err != nil {
				t.Fatal(err)
			}

			diff := c2 - c1
			if diff != tc.diff {
				t.Errorf(
					"Link count diff %d, want %d",
					diff,
					tc.diff,
				)
			}
		})
	}
}

type testClock struct{}

func (t testClock) Now() time.Time {
	return time.Date(2006, time.January, 2, 15, 4, 5, 0, t.location())
}

func (t testClock) location() *time.Location {
	l, err := time.LoadLocation("MST")
	if err != nil {
		panic(err)
	}
	return l
}

func seedTestDatastore(c context.Context) error {
	k := datastore.NewKey(c, "Link", "example", 0, nil)
	l := &Link{Location: "http://www.example.com/example"}
	if _, err := datastore.Put(c, k, l); err != nil {
		return err
	}

	k = datastore.NewKey(c, "User", "42", 0, nil)
	u := &User{
		Email:   "jane@example.com",
		Picture: "http://www.example.com/example.jpg",
	}
	_, err := datastore.Put(c, k, u)
	return err
}
