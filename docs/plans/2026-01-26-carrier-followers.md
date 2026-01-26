# Carrier Followers Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Track other carriers that appear near our configured carriers and provide commands to view follower statistics.

**Architecture:** Extend EDDN event processing to detect any carrier (not just ours) and record when external carriers are within a configurable distance of our carriers. Store follower data in a new database table. Add two new slash commands for viewing followers.

**Tech Stack:** Go, SQLite, discordgo, EDSM API for distance calculation

---

## Task 1: Add Configuration Option

**Files:**
- Modify: `core/settings.go`

**Step 1: Add followerDistanceThreshold to jsonData struct**

```go
// In jsonData struct, add after CarrierFlightLogChannelId:
FollowerDistanceThreshold float64 // Distance in ly to consider a carrier "following" (default 100)
```

**Step 2: Add accessor method**

```go
// FollowerDistanceThreshold returns the distance threshold for follower detection (default 100 ly)
func (s *SettingsStorage) FollowerDistanceThreshold() float64 {
	if s.data.FollowerDistanceThreshold <= 0 {
		return 100.0 // default
	}
	return s.data.FollowerDistanceThreshold
}
```

**Step 3: Verify build**

Run: `go build ./...`
Expected: No errors

**Step 4: Commit**

```bash
git add core/settings.go
git commit -m "feat: add followerDistanceThreshold config option"
```

---

## Task 2: Create Database Schema for Followers

**Files:**
- Modify: `core/database/carriers.go`

**Step 1: Add follower schema constant**

```go
// Add after carrierSchema constant:
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
```

**Step 2: Add CarrierFollower struct**

```go
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
```

**Step 3: Update InitializeCarrierTable to create followers table**

```go
// In InitializeCarrierTable(), add after carrier_state creation:
_, err = database.Exec(followerSchema)
if err != nil {
	core.LogErrorF("Failed to create carrier_followers table: %s", err)
}
```

**Step 4: Verify build**

Run: `go build ./...`
Expected: No errors

**Step 5: Commit**

```bash
git add core/database/carriers.go
git commit -m "feat: add carrier_followers database schema"
```

---

## Task 3: Add Database Functions for Followers

**Files:**
- Modify: `core/database/carriers.go`

**Step 1: Add UpsertCarrierFollower function**

```go
// UpsertCarrierFollower inserts or updates a follower record
// Returns true if this was a new sighting (location changed), false if just an update
func UpsertCarrierFollower(followerStationId, nearCarrier, system string, distance float64, eventTime int64) bool {
	// Check if follower exists and if location changed
	existing := FetchCarrierFollower(followerStationId)

	if existing == nil {
		// New follower
		_, err := database.Exec(`
			INSERT INTO carrier_followers
			(follower_station_id, last_near_carrier, last_system, last_distance, total_distance, times_seen, last_seen, first_seen)
			VALUES ($1, $2, $3, $4, $5, 1, $6, $6)`,
			followerStationId, nearCarrier, system, distance, distance, eventTime)
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
				last_near_carrier = $2,
				last_system = $3,
				last_distance = $4,
				total_distance = total_distance + $4,
				times_seen = times_seen + 1,
				last_seen = $5
			WHERE follower_station_id = $1`,
			followerStationId, nearCarrier, system, distance, eventTime)
		if err != nil {
			core.LogErrorF("Failed to update carrier follower: %s", err)
			return false
		}
		return true
	}

	// Same location - just update timestamp and distance (don't increment times_seen)
	_, err := database.Exec(`
		UPDATE carrier_followers SET
			last_near_carrier = $2,
			last_distance = $3,
			last_seen = $4
		WHERE follower_station_id = $1`,
		followerStationId, nearCarrier, distance, eventTime)
	if err != nil {
		core.LogErrorF("Failed to update carrier follower timestamp: %s", err)
	}
	return false
}
```

**Step 2: Add FetchCarrierFollower function**

```go
// FetchCarrierFollower retrieves a single follower by station ID
func FetchCarrierFollower(stationId string) *CarrierFollower {
	var follower CarrierFollower
	err := database.Get(&follower, "SELECT * FROM carrier_followers WHERE follower_station_id = $1", stationId)
	if err != nil {
		return nil
	}
	return &follower
}
```

**Step 3: Add FetchRecentFollowers function**

```go
// FetchRecentFollowers retrieves followers seen in the last N days with more than minSightings
// sortBy can be: "distance", "times", "recent"
func FetchRecentFollowers(days int, minSightings int, sortBy string) []CarrierFollower {
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
		WHERE last_seen >= $1 AND times_seen > $2
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
```

**Step 4: Add time import if needed**

Ensure `"time"` and `"fmt"` are in the imports.

**Step 5: Verify build**

Run: `go build ./...`
Expected: No errors

**Step 6: Commit**

```bash
git add core/database/carriers.go
git commit -m "feat: add database functions for carrier followers"
```

---

## Task 4: Add Follower Detection to EDDN Processing

**Files:**
- Modify: `core/services/eddn.go`

**Step 1: Add helper function to check if carrier is one of ours**

```go
// isOurCarrier checks if a station ID is one of our configured carriers
func isOurCarrier(stationId string) bool {
	return carrierCallsigns[stationId]
}
```

**Step 2: Add function to check and record followers**

```go
// checkAndRecordFollower checks if an external carrier is near any of our carriers
func checkAndRecordFollower(followerStationId, system string, eventTime int64) {
	// Don't track our own carriers as followers
	if isOurCarrier(followerStationId) {
		return
	}

	threshold := core.Settings.FollowerDistanceThreshold()

	// Get coordinates for the follower's system
	followerCoords, err := GetSystemCoords(system)
	if err != nil || followerCoords == nil {
		return // Unknown system, skip
	}

	// Check distance to each of our carriers
	for stationId := range carrierCallsigns {
		state := database.FetchCarrierState(stationId)
		if state == nil || state.CurrentSystem == nil || *state.CurrentSystem == "" {
			continue
		}

		ourCoords, err := GetSystemCoords(*state.CurrentSystem)
		if err != nil || ourCoords == nil {
			continue
		}

		distance := CalculateDistance(followerCoords, ourCoords)
		if distance >= 0 && distance <= threshold {
			// Within threshold - record as follower
			isNew := database.UpsertCarrierFollower(followerStationId, stationId, system, distance, eventTime)
			if isNew {
				core.LogDebugF("EDDN: Follower %s detected near %s at %s (%.1f ly)",
					followerStationId, stationId, system, distance)
			}
			return // Only record once per event (nearest carrier)
		}
	}
}
```

**Step 3: Update processJournalMessage to detect followers**

In `processJournalMessage`, after the existing switch cases, add detection for any carrier:

```go
// At the end of processJournalMessage, after the switch statement:

// Check for follower carriers in Location/FSDJump/Docked events
if msg.StationName != "" && isCarrierCallsign(msg.StationName) && !isOurCarrier(msg.StationName) {
	var eventTime int64
	if t, err := time.Parse(time.RFC3339, msg.Timestamp); err == nil {
		eventTime = t.Unix()
	} else if t, err := time.Parse("2006-01-02T15:04:05Z", msg.Timestamp); err == nil {
		eventTime = t.Unix()
	} else {
		eventTime = time.Now().Unix()
	}
	checkAndRecordFollower(msg.StationName, msg.StarSystem, eventTime)
}
```

**Step 4: Verify build**

Run: `go build ./...`
Expected: No errors

**Step 5: Commit**

```bash
git add core/services/eddn.go
git commit -m "feat: detect and record carrier followers from EDDN events"
```

---

## Task 5: Add Follower Service Functions

**Files:**
- Create: `core/services/followers.go`

**Step 1: Create the followers service file**

```go
package services

import (
	"fmt"
	"strings"
	"time"

	"GoBot/core/database"
)

// GetRecentFollowers returns formatted list of followers for display
func GetRecentFollowers(sortBy string) []database.CarrierFollower {
	// Last 7 days, more than 1 sighting
	return database.FetchRecentFollowers(7, 1, sortBy)
}

// GetFollowerInfo returns detailed info for a specific follower
func GetFollowerInfo(stationId string) *database.CarrierFollower {
	return database.FetchCarrierFollower(strings.ToUpper(stationId))
}

// FormatFollowerList formats followers for Discord display
func FormatFollowerList(followers []database.CarrierFollower, sortBy string) string {
	if len(followers) == 0 {
		return "No followers detected in the last 7 days (with multiple sightings)."
	}

	var sb strings.Builder
	sb.WriteString("**CARRIER FOLLOWERS** (last 7 days, 2+ sightings)\n")
	sb.WriteString(fmt.Sprintf("Sorted by: %s\n\n", sortBy))

	for _, f := range followers {
		avgDist := f.TotalDistance / float64(f.TimesSeen)
		sb.WriteString(fmt.Sprintf("**%s** - %d sightings\n", f.FollowerStationId, f.TimesSeen))
		sb.WriteString(fmt.Sprintf("  Last: %s (%.1f ly from %s)\n", f.LastSystem, f.LastDistance, f.LastNearCarrier))
		sb.WriteString(fmt.Sprintf("  Avg distance: %.1f ly | Last seen: <t:%d:R>\n\n", avgDist, f.LastSeen))
	}

	return sb.String()
}

// FormatFollowerInfo formats detailed info for a single follower
func FormatFollowerInfo(f *database.CarrierFollower) string {
	if f == nil {
		return "Carrier not found in follower database."
	}

	avgDist := f.TotalDistance / float64(f.TimesSeen)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**CARRIER INFO: %s**\n\n", f.FollowerStationId))
	sb.WriteString(fmt.Sprintf("**Current Location:** %s\n", f.LastSystem))
	sb.WriteString(fmt.Sprintf("**Last Distance:** %.1f ly (from %s)\n", f.LastDistance, f.LastNearCarrier))
	sb.WriteString(fmt.Sprintf("**Average Distance:** %.1f ly\n", avgDist))
	sb.WriteString(fmt.Sprintf("**Times Seen:** %d\n", f.TimesSeen))
	sb.WriteString(fmt.Sprintf("**First Seen:** <t:%d:F>\n", f.FirstSeen))
	sb.WriteString(fmt.Sprintf("**Last Seen:** <t:%d:F> (<t:%d:R>)\n", f.LastSeen, f.LastSeen))

	// Calculate tracking duration
	duration := time.Unix(f.LastSeen, 0).Sub(time.Unix(f.FirstSeen, 0))
	days := int(duration.Hours() / 24)
	if days > 0 {
		sb.WriteString(fmt.Sprintf("**Tracking Duration:** %d days\n", days))
	}

	return sb.String()
}
```

**Step 2: Verify build**

Run: `go build ./...`
Expected: No errors

**Step 3: Commit**

```bash
git add core/services/followers.go
git commit -m "feat: add follower service functions for formatting and retrieval"
```

---

## Task 6: Add /followers Slash Command

**Files:**
- Modify: `core/dispatch/handlers/carriers_slash.go`

**Step 1: Add /followers command definition**

In `carrierSlashCommands` slice, add:

```go
{
	Name:                     "followers",
	Description:              "List carriers following our fleet",
	DefaultMemberPermissions: &permissionAdministrator,
	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "sort",
			Description: "Sort by field",
			Required:    false,
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{Name: "Most Recent", Value: "recent"},
				{Name: "Most Sightings", Value: "times"},
				{Name: "Closest Distance", Value: "distance"},
			},
		},
	},
},
```

**Step 2: Add handler case in handleCarrierSlashAppCommand**

In the `switch data.Name` block, add:

```go
case "followers":
	if !canManageCarriersSlash(userID, i.ChannelID) {
		respond(s, i, "You don't have permission to view followers.", true)
		return
	}
	sortBy := "recent" // default
	if len(data.Options) > 0 {
		sortBy = data.Options[0].StringValue()
	}
	followers := services.GetRecentFollowers(sortBy)
	output := services.FormatFollowerList(followers, sortBy)
	respond(s, i, output, true)
```

**Step 3: Verify build**

Run: `go build ./...`
Expected: No errors

**Step 4: Commit**

```bash
git add core/dispatch/handlers/carriers_slash.go
git commit -m "feat: add /followers slash command"
```

---

## Task 7: Add /carrierinfo Slash Command

**Files:**
- Modify: `core/dispatch/handlers/carriers_slash.go`

**Step 1: Add /carrierinfo command definition**

In `carrierSlashCommands` slice, add:

```go
{
	Name:                     "carrierinfo",
	Description:              "Get detailed info about a carrier",
	DefaultMemberPermissions: &permissionAdministrator,
	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "carrier",
			Description: "Carrier station ID (e.g., ABC-123)",
			Required:    true,
		},
	},
},
```

**Step 2: Add handler case in handleCarrierSlashAppCommand**

In the `switch data.Name` block, add:

```go
case "carrierinfo":
	if !canManageCarriersSlash(userID, i.ChannelID) {
		respond(s, i, "You don't have permission to view carrier info.", true)
		return
	}
	stationId := strings.ToUpper(data.Options[0].StringValue())
	follower := services.GetFollowerInfo(stationId)
	output := services.FormatFollowerInfo(follower)
	respond(s, i, output, true)
```

**Step 3: Verify build**

Run: `go build ./...`
Expected: No errors

**Step 4: Commit**

```bash
git add core/dispatch/handlers/carriers_slash.go
git commit -m "feat: add /carrierinfo slash command"
```

---

## Task 8: Integration Test

**Files:**
- None (manual testing)

**Step 1: Update config file**

Add to your config JSON:
```json
"followerDistanceThreshold": 100.0
```

**Step 2: Build and run**

```bash
go build -o gobot . && ./gobot -c config-dev.json
```

**Step 3: Verify database table created**

Check logs for any errors during startup related to carrier_followers table.

**Step 4: Test /followers command**

In Discord, run `/followers` and `/followers sort:times`
Expected: Either "No followers detected..." or list of followers if any exist.

**Step 5: Test /carrierinfo command**

Run `/carrierinfo carrier:XXX-XXX` with a known carrier ID.
Expected: Detailed info or "Carrier not found" message.

**Step 6: Final commit**

```bash
git add config-dev.json  # if you want to commit the config change
git commit -m "feat: complete carrier followers feature"
```

---

## Summary

**New Config Options:**
- `followerDistanceThreshold` - Distance in ly to consider a carrier "following" (default: 100)

**New Database Table:**
- `carrier_followers` - Tracks external carriers seen near our fleet

**New Slash Commands:**
- `/followers [sort]` - List followers from last 7 days with 2+ sightings
- `/carrierinfo <carrier>` - Detailed info about a specific carrier

**Detection Logic:**
- EDDN events containing carrier callsigns (XXX-XXX format) are checked
- If carrier is NOT one of ours and is within threshold of any of our carriers, record it
- Only increment `times_seen` when the follower's location actually changes
