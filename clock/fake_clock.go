package clock

import "time"

type FakeClock struct{}

func (f FakeClock) Now() time.Time {
	return time.Date(2006, time.January, 2, 15, 4, 5, 0, f.location())
}

func (f FakeClock) location() *time.Location {
	l, err := time.LoadLocation("MST")
	if err != nil {
		panic(err)
	}
	return l
}
