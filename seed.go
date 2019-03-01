package main

import (
	"os"

	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
)

func seedData() error {
	ctx := appengine.BackgroundContext()

	k := datastore.NewKey(ctx, "User", os.Getenv("USER_ID"), 0, nil)
	u := &User{
		Email:   os.Getenv("USER_EMAIL"),
		Picture: os.Getenv("USER_PICTURE"),
	}
	if _, err := datastore.Put(ctx, k, u); err != nil {
		return err
	}

	k = datastore.NewKey(ctx, "Link", "example", 0, nil)
	l := &Link{Location: "http://www.example.com"}
	_, err := datastore.Put(ctx, k, l)

	return err
}
