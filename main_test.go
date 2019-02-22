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
		json []byte
		want error
	}{
		{
			name: "authorized email",
			json: []byte(`{"id":"42"}`),
			want: nil,
		},
		{
			name: "unauthorized email",
			json: []byte(`{"id":"1337"}`),
			want: errors.New("Failed to authorize user"),
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			if err = authorizeUserFromJSON(tc.json, ctx); err != nil {
				if err.Error() != tc.want.Error() {
					t.Errorf(
						`Error "%v", want "%v"`,
						err,
						tc.want,
					)
				}
			}
		})
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
