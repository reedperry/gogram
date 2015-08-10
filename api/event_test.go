package api

import (
	"appengine/aetest"
	"testing"
	"time"
)

func TestHasValidDuration(t *testing.T) {
	event := Event{
		ID:          "123myid",
		Name:        "Has Valid Duration",
		Description: "Test event start and end dates",
	}

	durationTests := []struct {
		start time.Time
		end   time.Time
		want  bool
	}{
		{
			want: false,
		},
		{
			start: time.Now(),
			want:  false,
		},
		{
			end:  time.Now().Add(time.Minute * 10),
			want: false,
		},
		{
			start: time.Now().Add(time.Hour),
			end:   time.Now().Add(time.Minute * 30),
			want:  false,
		},
		{
			start: time.Now(),
			end:   time.Now().Add(time.Hour),
			want:  true,
		},
		{
			start: time.Now().Add(-time.Hour * 48),
			end:   time.Now().Add(time.Hour * 48),
			want:  true,
		},
		{
			start: time.Now().Add(-time.Hour),
			end:   time.Now().Add(MAX_EVENT_LENGTH),
			want:  false,
		},
		{
			start: time.Now().Add(MAX_START_FUTURE).Add(time.Minute * 5),
			end:   time.Now().Add(MAX_START_FUTURE).Add(time.Hour * 48),
			want:  false,
		},
	}

	for _, test := range durationTests {
		event.Start = test.start
		event.End = test.end
		got := event.IsActive()
		if got != test.want {
			t.Errorf("HasValidDuration() returned %v for event with Start %v and End %v. Wanted %v.",
				got, event.Start, event.End, test.want)
		}
	}
}

func TestIsValidRequest(t *testing.T) {
	validTests := []struct {
		name        string
		description string
		start       time.Time
		end         time.Time
		want        bool
	}{
		{
			name: "Missing Fields",
			want: false,
		},
		{
			name:        "Event Name",
			description: "Missing End Date",
			start:       time.Now(),
			want:        false,
		},
		{
			description: "Missing Name",
			start:       time.Now(),
			end:         time.Now().Add(time.Hour * 24),
			want:        false,
		},
		{
			name:        "Event Name",
			description: "",
			start:       time.Now(),
			end:         time.Now().Add(time.Hour * 24),
			want:        false,
		},
		{
			name:        "Test Event",
			description: "A basic description.",
			start:       time.Now(),
			end:         time.Now().Add(time.Hour * 24),
			want:        true,
		},
		{
			name:        "Test Event",
			description: "Invalid - End after start.",
			start:       time.Now().Add(time.Hour * 24),
			end:         time.Now(),
			want:        false,
		},
	}

	event := Event{}
	for _, test := range validTests {
		event.Name = test.name
		event.Description = test.description
		event.Start = test.start
		event.End = test.end

		got := event.IsValidRequest()
		if got != test.want {
			t.Errorf("IsValidRequest() returned %v for event: %+v. Wanted %v.", got, event, test.want)
		}
	}
}

func TestIsValid(t *testing.T) {
	validTests := []struct {
		ID          string
		name        string
		description string
		start       time.Time
		end         time.Time
		creator     string
		created     time.Time
		want        bool
	}{
		{
			ID:          "123xyz",
			name:        "Missing Creator",
			description: "A description",
			start:       time.Now(),
			end:         time.Now().Add(time.Hour * 2),
			created:     time.Now(),
			want:        false,
		},
		{
			ID:          "123xyz",
			name:        "Missing Created",
			description: "A description",
			start:       time.Now(),
			end:         time.Now().Add(time.Hour * 2),
			creator:     "aUserID",
			want:        false,
		},
		{
			name:        "Missing ID",
			description: "A description",
			start:       time.Now(),
			end:         time.Now().Add(time.Hour * 2),
			creator:     "aUserID",
			created:     time.Now(),
			want:        false,
		},
		{
			ID:          "someID",
			name:        "Invalid start/end",
			description: "A description",
			start:       time.Now(),
			creator:     "aUserID",
			created:     time.Now(),
			want:        false,
		},
		{
			ID:          "someID",
			name:        "Invalid start/end",
			description: "A description",
			start:       time.Now(),
			end:         time.Now().Add(time.Hour * 5),
			creator:     "aUserID",
			created:     time.Now(),
			want:        true,
		},
	}

	event := Event{}
	for _, test := range validTests {
		event.ID = test.ID
		event.Name = test.name
		event.Description = test.description
		event.Start = test.start
		event.End = test.end
		event.Creator = test.creator
		event.Created = test.created

		got := event.IsValid()
		if got != test.want {
			t.Errorf("IsValid() returned %v for event: %+v. Wanted %v.", got, event, test.want)
		}
	}
}

func TestIsActive(t *testing.T) {
	event := Event{
		ID:          "123myid",
		Name:        "Event Name",
		Description: "Interesting Event",
	}

	activeTests := []struct {
		start time.Time
		end   time.Time
		want  bool
	}{
		{
			start: time.Now().Add(-time.Hour),
			end:   time.Now().Add(time.Hour),
			want:  true,
		},
		{
			start: time.Now().Add(time.Minute * 10),
			end:   time.Now().Add(time.Hour),
			want:  false,
		},
		{
			start: time.Now().Add(-time.Hour * 24),
			end:   time.Now().Add(-time.Hour * 3),
			want:  false,
		},
		{
			// Invalid - Start is after End, both in the past
			start: time.Now().Add(-time.Hour * 3),
			end:   time.Now().Add(-time.Hour * 24),
			want:  false,
		},
		{
			// Invalid - Start is after End, current time between them
			start: time.Now().Add(time.Hour * 3),
			end:   time.Now().Add(-time.Hour * 24),
			want:  false,
		},
	}

	for _, test := range activeTests {
		event.Start = test.start
		event.End = test.end
		got := event.IsActive()
		if got != test.want {
			t.Errorf("IsActive() returned %v for event with Start %v and End %v. Wanted %v.",
				got, event.Start, event.End, test.want)
		}
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
