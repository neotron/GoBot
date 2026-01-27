package database

import (
	"database/sql"
	"fmt"
	"time"

	"GoBot/core"
)

// CarrierState stores runtime state for a carrier
type CarrierState struct {
	StationId       string  `db:"station_id"`
	CurrentSystem   *string `db:"current_system"`
	SystemURL       *string `db:"system_url"`
	LocationUpdated *int64  `db:"location_updated"` // Last time we got a location update (even if same)
	LocationChanged *int64  `db:"location_changed"` // Last time location actually changed
	JumpTime        *int64  `db:"jump_time"`        // Manual jump time
	Destination     *string `db:"destination"`      // Manual destination
	Status          *string `db:"status"`
	PendingJumpDest *string `db:"pending_jump_dest"` // EDDN: scheduled jump destination
	PendingJumpTime *int64  `db:"pending_jump_time"` // EDDN: scheduled jump departure time
}

const carrierSchema = `
CREATE TABLE IF NOT EXISTS carrier_state (
	station_id TEXT PRIMARY KEY,
	current_system TEXT,
	system_url TEXT,
	location_updated INTEGER,
	location_changed INTEGER,
	jump_time INTEGER,
	destination TEXT,
	status TEXT
);
`

const followerSchema = `
CREATE TABLE IF NOT EXISTS carrier_followers (
	follower_station_id TEXT PRIMARY KEY,
	last_near_carrier TEXT NOT NULL,
	last_system TEXT NOT NULL,
	last_distance REAL NOT NULL,
	total_distance REAL NOT NULL DEFAULT 0,
	times_seen INTEGER NOT NULL DEFAULT 1,
	last_seen INTEGER NOT NULL,
	first_seen INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_followers_last_seen ON carrier_followers(last_seen);
CREATE INDEX IF NOT EXISTS idx_followers_times_seen ON carrier_followers(times_seen);
`

// CarrierFollower represents a carrier that has been seen near our carriers
type CarrierFollower struct {
	FollowerStationId string  `db:"follower_station_id"`
	LastNearCarrier   string  `db:"last_near_carrier"`
	LastSystem        string  `db:"last_system"`
	LastDistance      float64 `db:"last_distance"`
	TotalDistance     float64 `db:"total_distance"`
	TimesSeen         int     `db:"times_seen"`
	LastSeen          int64   `db:"last_seen"`
	FirstSeen         int64   `db:"first_seen"`
}

// InitializeCarrierTable creates the carrier_state table if it doesn't exist
func InitializeCarrierTable() {
	if database == nil {
		core.LogError("Database isn't open. Cannot initialize carrier table.")
		return
	}
	database.MustExec(carrierSchema)

	// Migrations for existing databases
	_, _ = database.Exec("ALTER TABLE carrier_state ADD COLUMN system_url TEXT")
	_, _ = database.Exec("ALTER TABLE carrier_state ADD COLUMN location_changed INTEGER")
	_, _ = database.Exec("ALTER TABLE carrier_state ADD COLUMN pending_jump_dest TEXT")
	_, _ = database.Exec("ALTER TABLE carrier_state ADD COLUMN pending_jump_time INTEGER")

	_, err := database.Exec(followerSchema)
	if err != nil {
		core.LogErrorF("Failed to create carrier_followers table: %s", err)
	}
}

// FetchCarrierState gets the current state for a carrier
func FetchCarrierState(stationId string) *CarrierState {
	if database == nil {
		core.LogError("Database isn't open.")
		return nil
	}
	state := CarrierState{}
	err := database.Get(&state, "SELECT * FROM carrier_state WHERE station_id=?", stationId)
	switch err {
	case sql.ErrNoRows:
		return nil
	case nil:
		return &state
	default:
		core.LogErrorF("Failed to fetch carrier state %s: %s", stationId, err)
		return nil
	}
}

// UpsertCarrierState creates or updates a carrier state record
func UpsertCarrierState(state *CarrierState) bool {
	_, err := executeAndCommit(func(tx *sql.Tx) (sql.Result, error) {
		return tx.Exec(`
			INSERT INTO carrier_state (station_id, current_system, system_url, location_updated, location_changed, jump_time, destination, status, pending_jump_dest, pending_jump_time)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(station_id) DO UPDATE SET
				current_system = excluded.current_system,
				system_url = excluded.system_url,
				location_updated = excluded.location_updated,
				location_changed = excluded.location_changed,
				jump_time = excluded.jump_time,
				destination = excluded.destination,
				status = excluded.status,
				pending_jump_dest = excluded.pending_jump_dest,
				pending_jump_time = excluded.pending_jump_time
		`, state.StationId, state.CurrentSystem, state.SystemURL, state.LocationUpdated, state.LocationChanged, state.JumpTime, state.Destination, state.Status, state.PendingJumpDest, state.PendingJumpTime)
	})
	if err != nil {
		core.LogErrorF("Failed to upsert carrier state: %s", err)
		return false
	}
	return true
}

// UpdateCarrierJumpTime sets the jump time for a carrier
func UpdateCarrierJumpTime(stationId string, jumpTime *int64) bool {
	state := FetchCarrierState(stationId)
	if state == nil {
		state = &CarrierState{StationId: stationId}
	}
	state.JumpTime = jumpTime
	return UpsertCarrierState(state)
}

// UpdateCarrierDestination sets the destination for a carrier
func UpdateCarrierDestination(stationId string, destination *string) bool {
	state := FetchCarrierState(stationId)
	if state == nil {
		state = &CarrierState{StationId: stationId}
	}
	state.Destination = destination
	return UpsertCarrierState(state)
}

// UpdateCarrierStatus sets the status for a carrier
func UpdateCarrierStatus(stationId string, status *string) bool {
	state := FetchCarrierState(stationId)
	if state == nil {
		state = &CarrierState{StationId: stationId}
	}
	state.Status = status
	return UpsertCarrierState(state)
}

// UpdateCarrierLocation sets the location, URL, and updates the timestamps
// Returns (success, locationChanged)
func UpdateCarrierLocation(stationId string, system string, systemURL string, timestamp int64) (bool, bool) {
	state := FetchCarrierState(stationId)
	if state == nil {
		state = &CarrierState{StationId: stationId}
	}

	// Check if location actually changed
	locationChanged := state.CurrentSystem == nil || *state.CurrentSystem != system

	state.CurrentSystem = &system
	if systemURL != "" {
		state.SystemURL = &systemURL
	}
	state.LocationUpdated = &timestamp

	// Only update location_changed timestamp if location actually changed
	if locationChanged {
		state.LocationChanged = &timestamp
	}

	return UpsertCarrierState(state), locationChanged
}

// UpdateCarrierPendingJump sets or clears the pending jump from EDDN
func UpdateCarrierPendingJump(stationId string, dest *string, jumpTime *int64) bool {
	state := FetchCarrierState(stationId)
	if state == nil {
		state = &CarrierState{StationId: stationId}
	}
	state.PendingJumpDest = dest
	state.PendingJumpTime = jumpTime
	return UpsertCarrierState(state)
}

// ClearCarrierPendingJump clears the pending jump (after jump completes or is cancelled)
func ClearCarrierPendingJump(stationId string) bool {
	return UpdateCarrierPendingJump(stationId, nil, nil)
}

// FetchCarrierFollower retrieves a single follower by station ID
func FetchCarrierFollower(stationId string) *CarrierFollower {
	if database == nil {
		return nil
	}
	var follower CarrierFollower
	err := database.Get(&follower, "SELECT * FROM carrier_followers WHERE follower_station_id = ?", stationId)
	switch err {
	case sql.ErrNoRows:
		return nil
	case nil:
		return &follower
	default:
		core.LogErrorF("Failed to fetch carrier follower %s: %s", stationId, err)
		return nil
	}
}

// UpsertCarrierFollower inserts or updates a follower record
// Returns true if this was a new sighting (location changed), false if just an update
func UpsertCarrierFollower(followerStationId, nearCarrier, system string, distance float64, eventTime int64) bool {
	if database == nil {
		return false
	}
	// Check if follower exists and if location changed
	existing := FetchCarrierFollower(followerStationId)

	if existing == nil {
		// New follower
		_, err := database.Exec(`
			INSERT INTO carrier_followers
			(follower_station_id, last_near_carrier, last_system, last_distance, total_distance, times_seen, last_seen, first_seen)
			VALUES (?, ?, ?, ?, ?, 1, ?, ?)`,
			followerStationId, nearCarrier, system, distance, distance, eventTime, eventTime)
		if err != nil {
			core.LogErrorF("Failed to insert carrier follower: %s", err)
			return false
		}
		return true
	}

	// Check if location actually changed (different system)
	locationChanged := existing.LastSystem != system

	if locationChanged {
		// Update with new sighting - increment times_seen and add to total_distance
		_, err := database.Exec(`
			UPDATE carrier_followers SET
				last_near_carrier = ?,
				last_system = ?,
				last_distance = ?,
				total_distance = total_distance + ?,
				times_seen = times_seen + 1,
				last_seen = ?
			WHERE follower_station_id = ?`,
			nearCarrier, system, distance, distance, eventTime, followerStationId)
		if err != nil {
			core.LogErrorF("Failed to update carrier follower: %s", err)
			return false
		}
		return true
	}

	// Same location - just update timestamp and distance (don't increment times_seen)
	_, err := database.Exec(`
		UPDATE carrier_followers SET
			last_near_carrier = ?,
			last_distance = ?,
			last_seen = ?
		WHERE follower_station_id = ?`,
		nearCarrier, distance, eventTime, followerStationId)
	if err != nil {
		core.LogErrorF("Failed to update carrier follower timestamp: %s", err)
	}
	return false
}

// FetchRecentFollowers retrieves followers seen in the last N days with more than minSightings
// sortBy can be: "distance", "times", "recent"
func FetchRecentFollowers(days int, minSightings int, sortBy string) []CarrierFollower {
	if database == nil {
		return nil
	}
	cutoff := time.Now().Unix() - int64(days*24*60*60)

	orderClause := "last_seen DESC" // default: recent
	switch sortBy {
	case "distance":
		orderClause = "last_distance ASC"
	case "times":
		orderClause = "times_seen DESC"
	case "recent":
		orderClause = "last_seen DESC"
	}

	query := fmt.Sprintf(`
		SELECT * FROM carrier_followers
		WHERE last_seen >= ? AND times_seen > ?
		ORDER BY %s
		LIMIT 25`, orderClause)

	var followers []CarrierFollower
	err := database.Select(&followers, query, cutoff, minSightings)
	if err != nil {
		core.LogErrorF("Failed to fetch recent followers: %s", err)
		return nil
	}
	return followers
}
