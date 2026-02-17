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

const carrierStatsSchema = `
CREATE TABLE IF NOT EXISTS carrier_stats (
	station_id TEXT NOT NULL,
	week_start TEXT NOT NULL,
	jumps INTEGER DEFAULT 0,
	ly_jumped REAL DEFAULT 0,
	location_events INTEGER DEFAULT 0,
	docked_events INTEGER DEFAULT 0,
	PRIMARY KEY (station_id, week_start)
);
`

// ProximityAlert represents a user's proximity alert for a carrier jump near a system
type ProximityAlert struct {
	ID         int64   `db:"id"`
	UserID     string  `db:"user_id"`
	SystemName string  `db:"system_name"`
	DistanceLY float64 `db:"distance_ly"`
	CreatedAt  int64   `db:"created_at"`
}

const proximityAlertSchema = `
CREATE TABLE IF NOT EXISTS proximity_alerts (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id TEXT NOT NULL,
	system_name TEXT NOT NULL,
	distance_ly REAL NOT NULL,
	created_at INTEGER NOT NULL
);
`

// CarrierStats holds aggregated carrier activity statistics
type CarrierStats struct {
	Jumps          int     `db:"jumps"`
	LYJumped       float64 `db:"ly_jumped"`
	LocationEvents int     `db:"location_events"`
	DockedEvents   int     `db:"docked_events"`
}

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

	_, err = database.Exec(carrierStatsSchema)
	if err != nil {
		core.LogErrorF("Failed to create carrier_stats table: %s", err)
	}

	_, err = database.Exec(proximityAlertSchema)
	if err != nil {
		core.LogErrorF("Failed to create proximity_alerts table: %s", err)
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

// currentWeekStart returns the Monday of the current UTC week as "2006-01-02"
func currentWeekStart() string {
	now := time.Now().UTC()
	weekday := now.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	monday := now.AddDate(0, 0, -int(weekday-time.Monday))
	return monday.Format("2006-01-02")
}

// IncrementCarrierJump records a carrier jump with distance
func IncrementCarrierJump(stationId string, distanceLY float64) {
	if database == nil {
		return
	}
	week := currentWeekStart()
	_, err := database.Exec(`
		INSERT INTO carrier_stats (station_id, week_start, jumps, ly_jumped)
		VALUES (?, ?, 1, ?)
		ON CONFLICT(station_id, week_start) DO UPDATE SET
			jumps = jumps + 1,
			ly_jumped = ly_jumped + ?`,
		stationId, week, distanceLY, distanceLY)
	if err != nil {
		core.LogErrorF("Failed to increment carrier jump stats: %s", err)
	}
}

// IncrementCarrierLocationEvent records a Location event (player logged in near carrier)
func IncrementCarrierLocationEvent(stationId string) {
	if database == nil {
		return
	}
	week := currentWeekStart()
	_, err := database.Exec(`
		INSERT INTO carrier_stats (station_id, week_start, location_events)
		VALUES (?, ?, 1)
		ON CONFLICT(station_id, week_start) DO UPDATE SET
			location_events = location_events + 1`,
		stationId, week)
	if err != nil {
		core.LogErrorF("Failed to increment carrier location event stats: %s", err)
	}
}

// IncrementCarrierDockedEvent records a Docked event (player logged in while docked)
func IncrementCarrierDockedEvent(stationId string) {
	if database == nil {
		return
	}
	week := currentWeekStart()
	_, err := database.Exec(`
		INSERT INTO carrier_stats (station_id, week_start, docked_events)
		VALUES (?, ?, 1)
		ON CONFLICT(station_id, week_start) DO UPDATE SET
			docked_events = docked_events + 1`,
		stationId, week)
	if err != nil {
		core.LogErrorF("Failed to increment carrier docked event stats: %s", err)
	}
}

// GetCarrierStats returns total and current-week stats for a carrier
func GetCarrierStats(stationId string) (total CarrierStats, weekly CarrierStats) {
	if database == nil {
		return
	}

	// Total stats across all weeks
	err := database.Get(&total, `
		SELECT COALESCE(SUM(jumps), 0) as jumps,
			   COALESCE(SUM(ly_jumped), 0) as ly_jumped,
			   COALESCE(SUM(location_events), 0) as location_events,
			   COALESCE(SUM(docked_events), 0) as docked_events
		FROM carrier_stats WHERE station_id = ?`, stationId)
	if err != nil {
		core.LogErrorF("Failed to fetch total carrier stats: %s", err)
	}

	// Current week stats
	week := currentWeekStart()
	err = database.Get(&weekly, `
		SELECT COALESCE(jumps, 0) as jumps,
			   COALESCE(ly_jumped, 0) as ly_jumped,
			   COALESCE(location_events, 0) as location_events,
			   COALESCE(docked_events, 0) as docked_events
		FROM carrier_stats WHERE station_id = ? AND week_start = ?`, stationId, week)
	if err != nil && err != sql.ErrNoRows {
		core.LogErrorF("Failed to fetch weekly carrier stats: %s", err)
	}

	return
}

// CreateProximityAlert creates a new proximity alert and returns its ID
func CreateProximityAlert(userID, systemName string, distanceLY float64) (int64, error) {
	res, err := executeAndCommit(func(tx *sql.Tx) (sql.Result, error) {
		return tx.Exec(`INSERT INTO proximity_alerts (user_id, system_name, distance_ly, created_at) VALUES (?, ?, ?, ?)`,
			userID, systemName, distanceLY, time.Now().Unix())
	})
	if err != nil {
		return 0, fmt.Errorf("failed to create proximity alert: %w", err)
	}
	return res.LastInsertId()
}

// FetchProximityAlertsByUser returns all proximity alerts for a given user
func FetchProximityAlertsByUser(userID string) []ProximityAlert {
	if database == nil {
		return nil
	}
	var alerts []ProximityAlert
	err := database.Select(&alerts, "SELECT * FROM proximity_alerts WHERE user_id = ? ORDER BY created_at DESC", userID)
	if err != nil {
		core.LogErrorF("Failed to fetch proximity alerts for user %s: %s", userID, err)
		return nil
	}
	return alerts
}

// FetchAllProximityAlerts returns all active proximity alerts
func FetchAllProximityAlerts() []ProximityAlert {
	if database == nil {
		return nil
	}
	var alerts []ProximityAlert
	err := database.Select(&alerts, "SELECT * FROM proximity_alerts")
	if err != nil {
		core.LogErrorF("Failed to fetch all proximity alerts: %s", err)
		return nil
	}
	return alerts
}

// DeleteProximityAlert deletes a specific alert owned by a user. Returns true if deleted.
func DeleteProximityAlert(id int64, userID string) bool {
	res, err := executeAndCommit(func(tx *sql.Tx) (sql.Result, error) {
		return tx.Exec("DELETE FROM proximity_alerts WHERE id = ? AND user_id = ?", id, userID)
	})
	if err != nil {
		core.LogErrorF("Failed to delete proximity alert %d: %s", id, err)
		return false
	}
	rows, _ := res.RowsAffected()
	return rows > 0
}

// DeleteAllProximityAlerts deletes all alerts for a user. Returns the count deleted.
func DeleteAllProximityAlerts(userID string) int64 {
	res, err := executeAndCommit(func(tx *sql.Tx) (sql.Result, error) {
		return tx.Exec("DELETE FROM proximity_alerts WHERE user_id = ?", userID)
	})
	if err != nil {
		core.LogErrorF("Failed to delete all proximity alerts for user %s: %s", userID, err)
		return 0
	}
	rows, _ := res.RowsAffected()
	return rows
}

// DeleteProximityAlertByID deletes a single alert by ID (used internally after firing)
func DeleteProximityAlertByID(id int64) bool {
	res, err := executeAndCommit(func(tx *sql.Tx) (sql.Result, error) {
		return tx.Exec("DELETE FROM proximity_alerts WHERE id = ?", id)
	})
	if err != nil {
		core.LogErrorF("Failed to delete proximity alert by ID %d: %s", id, err)
		return false
	}
	rows, _ := res.RowsAffected()
	return rows > 0
}
