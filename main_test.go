package main

import (
	"net/http/httptest"
	"os"
	"testing"

	"google.golang.org/appengine"
	"google.golang.org/appengine/aetest"
	"google.golang.org/appengine/datastore"
)

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
			key := datastore.NewKey(ctx, "Link", "example", 0, nil)
			if _, err := datastore.Put(
				ctx,
				key,
				&Link{Location: "http://www.example.com/example"},
			); err != nil {
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
