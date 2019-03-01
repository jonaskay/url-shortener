package main

import "time"

type clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

type fakeClock struct{}

func (f fakeClock) Now() time.Time {
	return time.Date(2006, time.January, 2, 15, 4, 5, 0, f.location())
}

func (f fakeClock) location() *time.Location {
	l, err := time.LoadLocation("MST")
	if err != nil {
		panic(err)
	}
	return l
}
