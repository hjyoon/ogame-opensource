package publicsite

import "testing"

func TestUniverseValidate(t *testing.T) {
	valid := Universe{
		Number:     1,
		Name:       "Universe 1",
		BaseURL:    "http://localhost:8888",
		Language:   "en",
		Speed:      1,
		FleetSpeed: 1,
		Status:     UniverseOnline,
	}

	if err := valid.Validate(); err != nil {
		t.Fatalf("expected valid universe: %v", err)
	}
	if !valid.Open() {
		t.Fatal("expected online universe to be open")
	}

	cases := map[string]Universe{
		"number":      {Name: "Universe 1", BaseURL: "http://localhost:8888", Language: "en", Speed: 1, FleetSpeed: 1, Status: UniverseOnline},
		"name":        {Number: 1, BaseURL: "http://localhost:8888", Language: "en", Speed: 1, FleetSpeed: 1, Status: UniverseOnline},
		"url":         {Number: 1, Name: "Universe 1", Language: "en", Speed: 1, FleetSpeed: 1, Status: UniverseOnline},
		"language":    {Number: 1, Name: "Universe 1", BaseURL: "http://localhost:8888", Speed: 1, FleetSpeed: 1, Status: UniverseOnline},
		"speed":       {Number: 1, Name: "Universe 1", BaseURL: "http://localhost:8888", Language: "en", FleetSpeed: 1, Status: UniverseOnline},
		"fleet_speed": {Number: 1, Name: "Universe 1", BaseURL: "http://localhost:8888", Language: "en", Speed: 1, Status: UniverseOnline},
		"status":      {Number: 1, Name: "Universe 1", BaseURL: "http://localhost:8888", Language: "en", Speed: 1, FleetSpeed: 1, Status: "unknown"},
	}
	for name, universe := range cases {
		t.Run(name, func(t *testing.T) {
			if err := universe.Validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}

	offline := valid
	offline.Status = UniverseOffline
	if offline.Open() {
		t.Fatal("expected offline universe to be closed")
	}
}
