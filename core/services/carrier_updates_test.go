package services

import (
	"testing"

	"GoBot/core"
)

func setupTestCarriers() {
	core.Settings.SetTestCarriers([]core.CarrierConfig{
		{StationId: "TBQ-6VX", Name: "Pillar of Chista"},
		{StationId: "V4V-2XZ", Name: "Fimbulthul"},
		{StationId: "V2W-85Z", Name: "DSEV Distant Suns"},
		{StationId: "ABC-123", Name: "DSEV Odysseus"},
	})
}

// --- isPlaceholder tests ---

func TestIsPlaceholder_ExactMatches(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"", true},
		{"TBD", true},
		{"tbd", true},
		{"TBA", true},
		{"tba", true},
		{"N/A", true},
		{"n/a", true},
		{"pending", true},
		{"Pending", true},
		{"---", true},
		{"???", true},
		{"underway", true},
		{"Underway", true},
		{"[processing]", true},
		{"[error]", true},
		{"[Processing]", true},
		{"[Error]", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := isPlaceholder(tt.input); got != tt.want {
				t.Errorf("isPlaceholder(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsPlaceholder_PartialMatches(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"in transit", true},
		{"In Transit", true},
		{"carrier in transit", true},
		{"in progress", true},
		{"update in progress", true},
		{"processing", true},
		{"still processing", true},
		{"update pending", true},
		{"jump pending", true},
		{"underway now", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := isPlaceholder(tt.input); got != tt.want {
				t.Errorf("isPlaceholder(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsPlaceholder_NotPlaceholder(t *testing.T) {
	tests := []string{
		"Thuecheae MT-Q e5-8",
		"Sol",
		"Waypoint 3",
		"9th Feb, 16:00 UTC",
		"1737399000",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			if isPlaceholder(input) {
				t.Errorf("isPlaceholder(%q) = true, want false", input)
			}
		})
	}
}

// --- isClearValue tests ---

func TestIsClearValue(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"None", true},
		{"none", true},
		{"NONE", true},
		{"None / Not Scheduled", true},
		{"none/not scheduled", true},
		{"Not Scheduled", true},
		{"not scheduled", true},
		// Not clear values
		{"TBD", false},
		{"pending", false},
		{"Thuecheae MT-Q e5-8", false},
		{"Waypoint 3", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := isClearValue(tt.input); got != tt.want {
				t.Errorf("isClearValue(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// --- ExtractProcGenSystemName tests ---

func TestExtractProcGenSystemName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "clean system name",
			input: "Thuecheae MT-Q e5-8",
			want:  "Thuecheae MT-Q e5-8",
		},
		{
			name:  "with heading towards prefix",
			input: "Heading towards Thuecheae MT-Q e5-8 (where the crow flies)",
			want:  "Thuecheae MT-Q e5-8",
		},
		{
			name:  "multi-word region name",
			input: "Blu Thua AI-A c14-8",
			want:  "Blu Thua AI-A c14-8",
		},
		{
			name:  "multi-word region with prefix text",
			input: "near Pru Aescs NC-M d7-192",
			want:  "Pru Aescs NC-M d7-192",
		},
		{
			name:  "another clean system",
			input: "Pyriveae EC-K b41-3",
			want:  "Pyriveae EC-K b41-3",
		},
		{
			name:  "no procgen name",
			input: "Sol",
			want:  "",
		},
		{
			name:  "no procgen name waypoint",
			input: "Waypoint 3",
			want:  "",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "sector mass only no region",
			input: "near AB-C d1-2",
			want:  "AB-C d1-2",
		},
		{
			name:  "large mass code numbers",
			input: "Eoch Pruae HH-V e2-4718",
			want:  "Eoch Pruae HH-V e2-4718",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractProcGenSystemName(tt.input)
			if got != tt.want {
				t.Errorf("ExtractProcGenSystemName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- ParseCarrierUpdates tests ---

func TestParseCarrierUpdates_DestinationPlaceholderCleared(t *testing.T) {
	setupTestCarriers()

	tests := []struct {
		name    string
		dest    string
		wantNil bool   // true if destination should not be set
		wantVal string // expected value if set ("" means clear)
	}{
		{"question marks", "???", false, ""},
		{"tbd", "TBD", false, ""},
		{"tba", "tba", false, ""},
		{"dashes", "---", false, ""},
		{"error", "[error]", false, ""},
		{"processing", "[processing]", false, ""},
		{"none", "None", false, ""},
		{"not scheduled", "Not Scheduled", false, ""},
		{"valid system", "Thuecheae MT-Q e5-8", false, "Thuecheae MT-Q e5-8"},
		{"waypoint name", "Waypoint 3", false, "Waypoint 3"},
		{"waypoint tba suffix", "Waypoint 3 (tba)", false, "Waypoint 3 (tba)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := "Carrier: Pillar of Chista TBQ-6VX\nDestination System: " + tt.dest
			updates := ParseCarrierUpdates(content)

			if len(updates) != 1 {
				t.Fatalf("expected 1 update, got %d", len(updates))
			}

			u := updates[0]
			if tt.wantNil {
				if u.Destination != nil {
					t.Errorf("expected Destination to be nil, got %q", *u.Destination)
				}
			} else {
				if u.Destination == nil {
					t.Fatal("expected Destination to be set, got nil")
				}
				if *u.Destination != tt.wantVal {
					t.Errorf("expected Destination=%q, got %q", tt.wantVal, *u.Destination)
				}
			}
		})
	}
}

func TestParseCarrierUpdates_DepartureCleared(t *testing.T) {
	setupTestCarriers()

	tests := []struct {
		name      string
		departure string
		wantNil   bool  // true if departure should not be set
		wantZero  bool  // true if departure should be 0 (cleared)
		wantGt0   bool  // true if departure should be > 0 (parsed time)
	}{
		{"tbd", "TBD", false, true, false},
		{"tba", "tba", false, true, false},
		{"update pending", "update pending", false, true, false},
		{"pending", "pending", false, true, false},
		{"none", "None", false, true, false},
		{"not scheduled", "Not Scheduled", false, true, false},
		{"dashes", "---", false, true, false},
		{"question marks", "???", false, true, false},
		{"unparseable text", "sometime next week", false, true, false},
		{"valid time", "9th Feb, 16:00 UTC", false, false, true},
		{"valid timestamp", "1737399000", false, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := "Carrier: Fimbulthul (V4V-2XZ)\nDeparture: " + tt.departure
			updates := ParseCarrierUpdates(content)

			if len(updates) != 1 {
				t.Fatalf("expected 1 update, got %d", len(updates))
			}

			u := updates[0]
			if tt.wantNil {
				if u.Departure != nil {
					t.Errorf("expected Departure to be nil, got %d", *u.Departure)
				}
				return
			}

			if u.Departure == nil {
				t.Fatal("expected Departure to be set, got nil")
			}

			if tt.wantZero && *u.Departure != 0 {
				t.Errorf("expected Departure=0 (cleared), got %d", *u.Departure)
			}
			if tt.wantGt0 && *u.Departure <= 0 {
				t.Errorf("expected Departure>0, got %d", *u.Departure)
			}
		})
	}
}

func TestParseCarrierUpdates_MultipleCarriers(t *testing.T) {
	setupTestCarriers()

	content := `Carrier: DSEV Odysseus
Current System: Thuecheae MT-Q e5-8
Destination System: ???
Departure: update pending
Tritium Buy Orders: None

Carrier: Pillar of Chista TBQ-6VX
Current System: Pyriveae EC-K b41-3
Destination System: Thuecheae MT-Q e5-8
Departure: TBD
Tritium Buy Orders: None

Carrier: DSEV Distant Suns - V2W-85Z
Current System: Thuecheae MT-Q e5-8
Destination System: Waypoint 3 (tba)
Departure: 9th Feb, 16:00 UTC
Tritium Buy Orders: None.

Carrier: Fimbulthul (V4V-2XZ)
Current System: Thuecheae MT-Q e5-8
Destination System: ---
Departure: tbd`

	updates := ParseCarrierUpdates(content)

	if len(updates) != 4 {
		t.Fatalf("expected 4 updates, got %d", len(updates))
	}

	// Carrier 1: DSEV Odysseus - dest ??? (clear), departure "update pending" (clear)
	u := updates[0]
	if u.StationId != "ABC-123" {
		t.Errorf("update[0] StationId = %q, want ABC-123", u.StationId)
	}
	if u.Destination == nil || *u.Destination != "" {
		t.Errorf("update[0] Destination should be cleared (empty), got %v", u.Destination)
	}
	if u.Departure == nil || *u.Departure != 0 {
		t.Errorf("update[0] Departure should be cleared (0), got %v", u.Departure)
	}

	// Carrier 2: Pillar of Chista - dest set, departure TBD (clear)
	u = updates[1]
	if u.StationId != "TBQ-6VX" {
		t.Errorf("update[1] StationId = %q, want TBQ-6VX", u.StationId)
	}
	if u.Destination == nil || *u.Destination != "Thuecheae MT-Q e5-8" {
		t.Errorf("update[1] Destination = %v, want 'Thuecheae MT-Q e5-8'", u.Destination)
	}
	if u.Departure == nil || *u.Departure != 0 {
		t.Errorf("update[1] Departure should be cleared (0), got %v", u.Departure)
	}

	// Carrier 3: DSEV Distant Suns - dest "Waypoint 3 (tba)" (set as-is), departure parsed
	u = updates[2]
	if u.StationId != "V2W-85Z" {
		t.Errorf("update[2] StationId = %q, want V2W-85Z", u.StationId)
	}
	if u.Destination == nil || *u.Destination != "Waypoint 3 (tba)" {
		t.Errorf("update[2] Destination = %v, want 'Waypoint 3 (tba)'", u.Destination)
	}
	if u.Departure == nil || *u.Departure <= 0 {
		t.Errorf("update[2] Departure should be > 0 (parsed time), got %v", u.Departure)
	}

	// Carrier 4: Fimbulthul - dest --- (clear), departure tbd (clear)
	u = updates[3]
	if u.StationId != "V4V-2XZ" {
		t.Errorf("update[3] StationId = %q, want V4V-2XZ", u.StationId)
	}
	if u.Destination == nil || *u.Destination != "" {
		t.Errorf("update[3] Destination should be cleared (empty), got %v", u.Destination)
	}
	if u.Departure == nil || *u.Departure != 0 {
		t.Errorf("update[3] Departure should be cleared (0), got %v", u.Departure)
	}
}

func TestParseCarrierUpdates_MatchByName(t *testing.T) {
	setupTestCarriers()

	content := "Carrier: DSEV Odysseus\nDestination System: Colonia\nDeparture: None"
	updates := ParseCarrierUpdates(content)

	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}

	if updates[0].StationId != "ABC-123" {
		t.Errorf("expected StationId=ABC-123 (matched by name), got %q", updates[0].StationId)
	}
	if updates[0].Destination == nil || *updates[0].Destination != "Colonia" {
		t.Errorf("expected Destination=Colonia, got %v", updates[0].Destination)
	}
	if updates[0].Departure == nil || *updates[0].Departure != 0 {
		t.Errorf("expected Departure=0 (cleared via None), got %v", updates[0].Departure)
	}
}

func TestParseCarrierUpdates_UnknownCarrierIgnored(t *testing.T) {
	setupTestCarriers()

	content := "Carrier: Unknown Ship XYZ-999\nDestination System: Sol"
	updates := ParseCarrierUpdates(content)

	if len(updates) != 0 {
		t.Errorf("expected 0 updates for unknown carrier, got %d", len(updates))
	}
}

func TestParseCarrierUpdates_NoCarrierPrefix(t *testing.T) {
	setupTestCarriers()

	content := "Some random text without carrier blocks"
	updates := ParseCarrierUpdates(content)

	if len(updates) != 0 {
		t.Errorf("expected 0 updates, got %d", len(updates))
	}
}