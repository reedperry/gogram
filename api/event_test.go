package api

import (
	"appengine/aetest"
	"testing"
	"time"
)

func TestIsActive(t *testing.T) {
	event := Event{
		ID:          "123myid",
		Name:        "Event Name",
		Description: "Interesting Event",
		Start:       time.Now().Add(-time.Hour),
		End:         time.Now().Add(time.Hour),
	}

	if !event.IsActive() {
		t.Errorf("IsActive() returned false for event with Start %v and End %v. Wanted true.", event.Start, event.End)
	}

	event.Start = time.Now().Add(time.Minute * 10)
	event.End = time.Now().Add(time.Hour)
	if event.IsActive() {
		t.Errorf("IsActive() returned true for event with Start %v and End %v. Wanted false.", event.Start, event.End)
	}

	event.Start = time.Now().Add(-time.Hour * 24)
	event.End = time.Now().Add(-time.Hour * 3)
	if event.IsActive() {
		t.Errorf("IsActive() returned true for event with Start %v and End %v. Wanted false.", event.Start, event.End)
	}

	// Invalid - Start is after End, both in the past
	event.End = time.Now().Add(-time.Hour * 24)
	event.Start = time.Now().Add(-time.Hour * 3)
	if event.IsActive() {
		t.Errorf("IsActive() returned true for event with Start %v and End %v. Wanted false.", event.Start, event.End)
	}

	// Invalid - Start is after End, current time is between them
	event.End = time.Now().Add(-time.Hour * 24)
	event.Start = time.Now().Add(time.Hour * 3)
	if event.IsActive() {
		t.Errorf("IsActive() returned true for event with Start %v and End %v. Wanted false.", event.Start, event.End)
	}

}

func TestCreateEventKeyID(t *testing.T) {
	c, err := aetest.NewContext(nil)
	if err != nil {
		t.Fatal(err)
	}

	if createEventKeyID("123abc", c) != "event:123abc" {
		t.Error("createEventKey(\"123abc\") failed")
	}

	if createEventKeyID("", c) != "event:" {
		t.Error("createEventKey(\"\") failed")
	}
}

func TestValidFeedOrder(t *testing.T) {
	var orderTests = []struct {
		order string
		want  bool
	}{
		{"Bogus", false},
		{"", false},
		{"-", false},
		{"Created", true},
		{"-Created", true},
		{"End", true},
		{"-End", true},
	}

	for _, test := range orderTests {
		got := validFeedOrder(test.order)
		if got != test.want {
			t.Errorf("validFeedOrder(\"%v\") failed: Wanted %v, got %v.", test.order, test.want, got)
		}
	}
}
