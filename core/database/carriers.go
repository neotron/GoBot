package database

import (
	"database/sql"

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
}

// FetchCarrierState gets the current state for a carrier
func FetchCarrierState(stationId string) *CarrierState {
	if database == nil {
		core.LogError("Database isn't open.")
		return nil
	}
	state := CarrierState{}
	err := database.Get(&state, "SELECT * FROM carrier_state WHERE station_id=$1", stationId)
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
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
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
