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
	_, err := datastore.Put(ctx, k, u)
	return err
}
