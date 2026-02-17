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

	// Create the proximity_alerts schema
	_, err = db.Exec(proximityAlertSchema)
	if err != nil {
		t.Fatalf("Failed to create proximity_alerts schema: %v", err)
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

func TestCreateProximityAlert(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	id, err := CreateProximityAlert("user1", "Sol", 50.0, "")
	if err != nil {
		t.Fatalf("Failed to create proximity alert: %v", err)
	}
	if id <= 0 {
		t.Errorf("Expected positive ID, got %d", id)
	}

	// Test with carrier filter
	id2, err := CreateProximityAlert("user1", "Alpha Centauri", 100.0, "XYZ-123")
	if err != nil {
		t.Fatalf("Failed to create carrier-filtered proximity alert: %v", err)
	}
	if id2 <= id {
		t.Errorf("Expected id2 > id, got id=%d id2=%d", id, id2)
	}

	// Verify carrier ID was stored
	alerts := FetchProximityAlertsByUser("user1")
	if len(alerts) != 2 {
		t.Fatalf("Expected 2 alerts, got %d", len(alerts))
	}
	// Alerts are ordered by created_at DESC, so newest first
	foundFiltered := false
	foundAll := false
	for _, a := range alerts {
		if a.CarrierID == "XYZ-123" {
			foundFiltered = true
		}
		if a.CarrierID == "" {
			foundAll = true
		}
	}
	if !foundFiltered {
		t.Error("Expected to find alert with CarrierID='XYZ-123'")
	}
	if !foundAll {
		t.Error("Expected to find alert with empty CarrierID (all carriers)")
	}
}

func TestFetchProximityAlertsByUser(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Create alerts for two different users
	CreateProximityAlert("user1", "Sol", 50.0, "")
	CreateProximityAlert("user1", "Alpha Centauri", 100.0, "")
	CreateProximityAlert("user2", "Sirius", 75.0, "")

	// Verify user1 only sees their own alerts
	alerts1 := FetchProximityAlertsByUser("user1")
	if len(alerts1) != 2 {
		t.Fatalf("Expected 2 alerts for user1, got %d", len(alerts1))
	}
	for _, a := range alerts1 {
		if a.UserID != "user1" {
			t.Errorf("Expected UserID='user1', got '%s'", a.UserID)
		}
	}

	// Verify user2 only sees their own alerts
	alerts2 := FetchProximityAlertsByUser("user2")
	if len(alerts2) != 1 {
		t.Fatalf("Expected 1 alert for user2, got %d", len(alerts2))
	}
	if alerts2[0].SystemName != "Sirius" {
		t.Errorf("Expected SystemName='Sirius', got '%s'", alerts2[0].SystemName)
	}
}

func TestFetchAllProximityAlerts(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	CreateProximityAlert("user1", "Sol", 50.0, "")
	CreateProximityAlert("user2", "Alpha Centauri", 100.0, "ABC-001")
	CreateProximityAlert("user3", "Sirius", 75.0, "")

	alerts := FetchAllProximityAlerts()
	if len(alerts) != 3 {
		t.Errorf("Expected 3 alerts total, got %d", len(alerts))
	}
}

func TestProximityAlertCarrierFilter(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Create alerts: one for all carriers, one for a specific carrier
	CreateProximityAlert("user1", "Sol", 50.0, "")
	CreateProximityAlert("user1", "Alpha Centauri", 100.0, "XYZ-999")
	CreateProximityAlert("user2", "Sirius", 75.0, "ABC-001")

	alerts := FetchAllProximityAlerts()
	if len(alerts) != 3 {
		t.Fatalf("Expected 3 alerts, got %d", len(alerts))
	}

	// Verify CarrierID values are stored and returned correctly
	carrierIDs := make(map[string]string) // systemName -> carrierID
	for _, a := range alerts {
		carrierIDs[a.SystemName] = a.CarrierID
	}

	if carrierIDs["Sol"] != "" {
		t.Errorf("Expected Sol alert to have empty CarrierID, got '%s'", carrierIDs["Sol"])
	}
	if carrierIDs["Alpha Centauri"] != "XYZ-999" {
		t.Errorf("Expected Alpha Centauri alert CarrierID='XYZ-999', got '%s'", carrierIDs["Alpha Centauri"])
	}
	if carrierIDs["Sirius"] != "ABC-001" {
		t.Errorf("Expected Sirius alert CarrierID='ABC-001', got '%s'", carrierIDs["Sirius"])
	}
}

func TestDeleteProximityAlert(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	id, _ := CreateProximityAlert("user1", "Sol", 50.0, "")

	// Deleting another user's alert should return false
	deleted := DeleteProximityAlert(id, "user2")
	if deleted {
		t.Error("Expected false when deleting another user's alert")
	}

	// Alert should still exist
	alerts := FetchProximityAlertsByUser("user1")
	if len(alerts) != 1 {
		t.Fatalf("Expected alert to still exist, got %d alerts", len(alerts))
	}

	// Deleting own alert should return true
	deleted = DeleteProximityAlert(id, "user1")
	if !deleted {
		t.Error("Expected true when deleting own alert")
	}

	// Alert should be gone
	alerts = FetchProximityAlertsByUser("user1")
	if len(alerts) != 0 {
		t.Errorf("Expected 0 alerts after delete, got %d", len(alerts))
	}
}

func TestDeleteAllProximityAlerts(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	CreateProximityAlert("user1", "Sol", 50.0, "")
	CreateProximityAlert("user1", "Alpha Centauri", 100.0, "")
	CreateProximityAlert("user1", "Sirius", 75.0, "")
	// Another user's alert should not be affected
	CreateProximityAlert("user2", "Barnards Star", 25.0, "")

	count := DeleteAllProximityAlerts("user1")
	if count != 3 {
		t.Errorf("Expected 3 deleted, got %d", count)
	}

	// user1 should have no alerts
	alerts := FetchProximityAlertsByUser("user1")
	if len(alerts) != 0 {
		t.Errorf("Expected 0 alerts for user1 after delete all, got %d", len(alerts))
	}

	// user2's alert should still exist
	alerts2 := FetchProximityAlertsByUser("user2")
	if len(alerts2) != 1 {
		t.Errorf("Expected 1 alert for user2, got %d", len(alerts2))
	}
}

func TestDeleteProximityAlertByID(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	id, _ := CreateProximityAlert("user1", "Sol", 50.0, "")

	deleted := DeleteProximityAlertByID(id)
	if !deleted {
		t.Error("Expected true when deleting existing alert by ID")
	}

	// Alert should be gone
	alerts := FetchProximityAlertsByUser("user1")
	if len(alerts) != 0 {
		t.Errorf("Expected 0 alerts after delete, got %d", len(alerts))
	}

	// Deleting again should return false
	deleted = DeleteProximityAlertByID(id)
	if deleted {
		t.Error("Expected false when deleting non-existent alert")
	}
}
