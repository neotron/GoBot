package database

import (
	"os"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) func() {
	db, err := sqlx.Connect("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Create the follower schema
	_, err = db.Exec(followerSchema)
	if err != nil {
		t.Fatalf("Failed to create follower schema: %v", err)
	}

	// Set the package-level database
	database = db

	// Return cleanup function
	return func() {
		db.Close()
		database = nil
	}
}

func TestMain(m *testing.M) {
	// Run tests
	code := m.Run()
	os.Exit(code)
}

func TestUpsertCarrierFollower_NewFollower(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Insert a new follower
	isNew := UpsertCarrierFollower("ABC-123", "OUR-001", "Sol", 50.0, 1000)

	if !isNew {
		t.Error("Expected isNew=true for new follower, got false")
	}

	// Verify it was inserted
	follower := FetchCarrierFollower("ABC-123")
	if follower == nil {
		t.Fatal("Expected to find follower after insert, got nil")
	}

	if follower.FollowerStationId != "ABC-123" {
		t.Errorf("Expected FollowerStationId='ABC-123', got '%s'", follower.FollowerStationId)
	}
	if follower.LastNearCarrier != "OUR-001" {
		t.Errorf("Expected LastNearCarrier='OUR-001', got '%s'", follower.LastNearCarrier)
	}
	if follower.LastSystem != "Sol" {
		t.Errorf("Expected LastSystem='Sol', got '%s'", follower.LastSystem)
	}
	if follower.TimesSeen != 1 {
		t.Errorf("Expected TimesSeen=1, got %d", follower.TimesSeen)
	}
	if follower.LastDistance != 50.0 {
		t.Errorf("Expected LastDistance=50.0, got %f", follower.LastDistance)
	}
	if follower.TotalDistance != 50.0 {
		t.Errorf("Expected TotalDistance=50.0, got %f", follower.TotalDistance)
	}
}

func TestUpsertCarrierFollower_LocationChanged(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Insert initial follower
	UpsertCarrierFollower("ABC-123", "OUR-001", "Sol", 50.0, 1000)

	// Update with different location
	isNew := UpsertCarrierFollower("ABC-123", "OUR-001", "Alpha Centauri", 75.0, 2000)

	if !isNew {
		t.Error("Expected isNew=true for location change, got false")
	}

	// Verify the update
	follower := FetchCarrierFollower("ABC-123")
	if follower == nil {
		t.Fatal("Expected to find follower after update, got nil")
	}

	if follower.LastSystem != "Alpha Centauri" {
		t.Errorf("Expected LastSystem='Alpha Centauri', got '%s'", follower.LastSystem)
	}
	if follower.TimesSeen != 2 {
		t.Errorf("Expected TimesSeen=2 after location change, got %d", follower.TimesSeen)
	}
	if follower.LastDistance != 75.0 {
		t.Errorf("Expected LastDistance=75.0, got %f", follower.LastDistance)
	}
	// TotalDistance should be 50 + 75 = 125
	if follower.TotalDistance != 125.0 {
		t.Errorf("Expected TotalDistance=125.0, got %f", follower.TotalDistance)
	}
	if follower.LastSeen != 2000 {
		t.Errorf("Expected LastSeen=2000, got %d", follower.LastSeen)
	}
	// FirstSeen should not change
	if follower.FirstSeen != 1000 {
		t.Errorf("Expected FirstSeen=1000 (unchanged), got %d", follower.FirstSeen)
	}
}

func TestUpsertCarrierFollower_SameLocation(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Insert initial follower
	UpsertCarrierFollower("ABC-123", "OUR-001", "Sol", 50.0, 1000)

	// Update with same location but different time/distance
	isNew := UpsertCarrierFollower("ABC-123", "OUR-002", "Sol", 60.0, 2000)

	if isNew {
		t.Error("Expected isNew=false for same location, got true")
	}

	// Verify times_seen did NOT increment
	follower := FetchCarrierFollower("ABC-123")
	if follower == nil {
		t.Fatal("Expected to find follower, got nil")
	}

	if follower.TimesSeen != 1 {
		t.Errorf("Expected TimesSeen=1 (unchanged) for same location, got %d", follower.TimesSeen)
	}
	// But last_seen and last_distance should update
	if follower.LastSeen != 2000 {
		t.Errorf("Expected LastSeen=2000, got %d", follower.LastSeen)
	}
	if follower.LastDistance != 60.0 {
		t.Errorf("Expected LastDistance=60.0, got %f", follower.LastDistance)
	}
	// TotalDistance should NOT change for same location
	if follower.TotalDistance != 50.0 {
		t.Errorf("Expected TotalDistance=50.0 (unchanged), got %f", follower.TotalDistance)
	}
}

func TestUpsertCarrierFollower_MultipleLocationChanges(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Insert initial
	UpsertCarrierFollower("ABC-123", "OUR-001", "Sol", 10.0, 1000)

	// Change location 1
	UpsertCarrierFollower("ABC-123", "OUR-001", "Alpha Centauri", 20.0, 2000)

	// Change location 2
	UpsertCarrierFollower("ABC-123", "OUR-001", "Barnards Star", 30.0, 3000)

	// Same location (should not increment)
	UpsertCarrierFollower("ABC-123", "OUR-001", "Barnards Star", 35.0, 4000)

	// Change location 3
	UpsertCarrierFollower("ABC-123", "OUR-001", "Sirius", 40.0, 5000)

	follower := FetchCarrierFollower("ABC-123")
	if follower == nil {
		t.Fatal("Expected to find follower, got nil")
	}

	// Should have 4 sightings (initial + 3 location changes)
	if follower.TimesSeen != 4 {
		t.Errorf("Expected TimesSeen=4, got %d", follower.TimesSeen)
	}

	// Total distance: 10 + 20 + 30 + 40 = 100
	if follower.TotalDistance != 100.0 {
		t.Errorf("Expected TotalDistance=100.0, got %f", follower.TotalDistance)
	}

	if follower.LastSystem != "Sirius" {
		t.Errorf("Expected LastSystem='Sirius', got '%s'", follower.LastSystem)
	}
}

func TestFetchCarrierFollower_NotFound(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	follower := FetchCarrierFollower("NONEXISTENT")
	if follower != nil {
		t.Errorf("Expected nil for non-existent follower, got %+v", follower)
	}
}

func TestFetchRecentFollowers(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now().Unix()
	oldTime := now - (8 * 24 * 60 * 60) // 8 days ago
	recentTime := now - (1 * 24 * 60 * 60) // 1 day ago

	// Insert old follower with 1 sighting
	UpsertCarrierFollower("OLD-001", "OUR-001", "Sol", 50.0, oldTime)

	// Insert recent follower with 1 sighting (should not appear - needs 2+)
	UpsertCarrierFollower("NEW-001", "OUR-001", "Sol", 50.0, recentTime)

	// Insert recent follower with 2 sightings
	UpsertCarrierFollower("NEW-002", "OUR-001", "Sol", 50.0, recentTime)
	UpsertCarrierFollower("NEW-002", "OUR-001", "Alpha Centauri", 60.0, recentTime+100)

	// Insert recent follower with 3 sightings
	UpsertCarrierFollower("NEW-003", "OUR-001", "Sol", 30.0, recentTime)
	UpsertCarrierFollower("NEW-003", "OUR-001", "Alpha Centauri", 40.0, recentTime+100)
	UpsertCarrierFollower("NEW-003", "OUR-001", "Barnards Star", 50.0, recentTime+200)

	// Fetch recent followers (7 days, more than 1 sighting)
	followers := FetchRecentFollowers(7, 1, "recent")

	if len(followers) != 2 {
		t.Errorf("Expected 2 recent followers with 2+ sightings, got %d", len(followers))
		for _, f := range followers {
			t.Logf("  Found: %s with %d sightings", f.FollowerStationId, f.TimesSeen)
		}
	}

	// Check they are the right ones
	foundNew002 := false
	foundNew003 := false
	for _, f := range followers {
		if f.FollowerStationId == "NEW-002" {
			foundNew002 = true
			if f.TimesSeen != 2 {
				t.Errorf("NEW-002 should have 2 sightings, got %d", f.TimesSeen)
			}
		}
		if f.FollowerStationId == "NEW-003" {
			foundNew003 = true
			if f.TimesSeen != 3 {
				t.Errorf("NEW-003 should have 3 sightings, got %d", f.TimesSeen)
			}
		}
	}

	if !foundNew002 {
		t.Error("Expected to find NEW-002 in results")
	}
	if !foundNew003 {
		t.Error("Expected to find NEW-003 in results")
	}
}

func TestFetchRecentFollowers_SortByTimes(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now().Unix()

	// Insert followers with different sighting counts
	UpsertCarrierFollower("FEW-001", "OUR-001", "Sol", 50.0, now)
	UpsertCarrierFollower("FEW-001", "OUR-001", "Alpha", 50.0, now+1)

	UpsertCarrierFollower("MANY-001", "OUR-001", "Sol", 50.0, now)
	UpsertCarrierFollower("MANY-001", "OUR-001", "Alpha", 50.0, now+1)
	UpsertCarrierFollower("MANY-001", "OUR-001", "Beta", 50.0, now+2)
	UpsertCarrierFollower("MANY-001", "OUR-001", "Gamma", 50.0, now+3)

	followers := FetchRecentFollowers(7, 1, "times")

	if len(followers) < 2 {
		t.Fatalf("Expected at least 2 followers, got %d", len(followers))
	}

	// First should be MANY-001 (4 sightings)
	if followers[0].FollowerStationId != "MANY-001" {
		t.Errorf("Expected first follower to be MANY-001 (most sightings), got %s", followers[0].FollowerStationId)
	}
	if followers[0].TimesSeen != 4 {
		t.Errorf("Expected MANY-001 to have 4 sightings, got %d", followers[0].TimesSeen)
	}
}
