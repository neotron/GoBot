# Carrier Commands Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add fleet carrier tracking commands with both message and slash command interfaces.

**Architecture:** Config defines carriers and permissions; database stores runtime state (location, jump time, destination, status); shared service layer handles business logic; separate handlers for message and slash commands.

**Tech Stack:** Go, discordgo, sqlx/sqlite3, net/http for Inara scraping

---

## Task 1: Rename Config Field (cooldownWhitelistChannels → botChannels)

**Files:**
- Modify: `core/settings.go`
- Modify: `core/dispatch/handlers/custom.go`
- Modify: `config.json.example`

**Step 1: Update settings.go struct field**

In `core/settings.go`, change line 18:

```go
// Before:
CooldownWhitelistChannels    []string // Channel IDs exempt from cooldown (e.g., bot-spam channels)

// After:
BotChannels    []string // Channel IDs for bot-spam (cooldown exempt, carriers reply in channel)
```

**Step 2: Update settings.go getter functions**

Replace the two functions at lines 78-90:

```go
// BotChannels returns the list of bot channel IDs
func (s *SettingsStorage) BotChannels() []string {
	return s.data.BotChannels
}

// IsBotChannel checks if a channel ID is a bot channel
func (s *SettingsStorage) IsBotChannel(channelID string) bool {
	for _, id := range s.data.BotChannels {
		if id == channelID {
			return true
		}
	}
	return false
}
```

**Step 3: Update custom.go to use new function name**

In `core/dispatch/handlers/custom.go`, find `IsChannelCooldownWhitelisted` and replace with `IsBotChannel`:

```go
// Before (around line 377):
if !m.IsPM && !core.Settings.IsChannelCooldownWhitelisted(m.ChannelID) {

// After:
if !m.IsPM && !core.Settings.IsBotChannel(m.ChannelID) {
```

**Step 4: Update config.json.example**

Change field name:

```json
"botChannels": [
  "channel ID for bot-spam etc"
]
```

**Step 5: Build to verify**

Run: `go build ./...`
Expected: Build succeeds

**Step 6: Commit**

```bash
git add core/settings.go core/dispatch/handlers/custom.go config.json.example
git commit -m "refactor: rename cooldownWhitelistChannels to botChannels

More general name for channels where bot can be verbose.
Used for cooldown bypass and carrier replies."
```

---

## Task 2: Add Carrier Config Structure

**Files:**
- Modify: `core/settings.go`
- Modify: `config.json.example`

**Step 1: Add carrier struct and config fields**

In `core/settings.go`, add after the imports (around line 9):

```go
// CarrierConfig defines a fleet carrier from config
type CarrierConfig struct {
	StationId string `json:"stationId"` // Carrier callsign e.g. "W7H-6DZ"
	InaraId   int    `json:"inaraId"`   // Inara station ID for location lookup
	Name      string `json:"name"`      // Display name e.g. "DSEV Odysseus"
}
```

**Step 2: Add fields to jsonData struct**

Add these fields to the `jsonData` struct:

```go
type jsonData struct {
	Development               bool
	AuthToken                 string
	CommandPrefix             string
	Database                  string
	ResourceDirectory         string
	OwnerIds                  []string
	CustomCommandCooldown     int              // Cooldown in seconds between same custom command uses (0 = no cooldown)
	BotChannels               []string         // Channel IDs for bot-spam (cooldown exempt, carriers reply in channel)
	CarrierOwnerIds           []string         // Discord user IDs who can manage carriers
	Carriers                  []CarrierConfig  // Fleet carrier definitions
	CarrierLocationCacheMinutes int            // Minutes to cache Inara location (default 20)
}
```

**Step 3: Add getter methods**

Add after the existing getters:

```go
// CarrierOwnerIds returns the list of carrier commander Discord user IDs
func (s *SettingsStorage) CarrierOwnerIds() []string {
	return s.data.CarrierOwnerIds
}

// IsCarrierOwner checks if a user ID can manage carriers
func (s *SettingsStorage) IsCarrierOwner(userID string) bool {
	for _, id := range s.data.CarrierOwnerIds {
		if id == userID {
			return true
		}
	}
	return false
}

// Carriers returns the list of configured carriers
func (s *SettingsStorage) Carriers() []CarrierConfig {
	return s.data.Carriers
}

// GetCarrierByStationId finds a carrier config by station ID
func (s *SettingsStorage) GetCarrierByStationId(stationId string) *CarrierConfig {
	for i := range s.data.Carriers {
		if s.data.Carriers[i].StationId == stationId {
			return &s.data.Carriers[i]
		}
	}
	return nil
}

// CarrierLocationCacheMinutes returns cache duration (default 20)
func (s *SettingsStorage) CarrierLocationCacheMinutes() int {
	if s.data.CarrierLocationCacheMinutes <= 0 {
		return 20
	}
	return s.data.CarrierLocationCacheMinutes
}
```

**Step 4: Update config.json.example**

Add carrier configuration:

```json
{
  "development": true,
  "authToken": "BOT TOKEN",
  "commandPrefix": "#",
  "database_directory": "DATABASE DIR",
  "resource_directory": "RESOURCE DIR",
  "owner_ids": [
    "Discord ID",
    "of people capable of bot admin"
  ],
  "customCommandCooldown": 20,
  "botChannels": [
    "channel ID for bot-spam etc"
  ],
  "carrierOwnerIds": [
    "Discord user ID of carrier commanders"
  ],
  "carriers": [
    {
      "stationId": "W7H-6DZ",
      "inaraId": 184006,
      "name": "DSEV Odysseus"
    },
    {
      "stationId": "V2W-85Z",
      "inaraId": 0,
      "name": "DSEV Distant Suns"
    },
    {
      "stationId": "V4V-2XZ",
      "inaraId": 0,
      "name": "DSEC Fimbulthul"
    },
    {
      "stationId": "TBQ-6VX",
      "inaraId": 0,
      "name": "Pillar of Chista"
    }
  ],
  "carrierLocationCacheMinutes": 20
}
```

**Step 5: Build to verify**

Run: `go build ./...`
Expected: Build succeeds

**Step 6: Commit**

```bash
git add core/settings.go config.json.example
git commit -m "feat: add carrier configuration structure

- CarrierConfig struct for carrier definitions
- carrierOwnerIds for carrier commander permissions
- carriers array with stationId, inaraId, name
- carrierLocationCacheMinutes for Inara cache duration"
```

---

## Task 3: Add Carrier Database Schema

**Files:**
- Create: `core/database/carriers.go`

**Step 1: Create carriers.go with schema and struct**

Create new file `core/database/carriers.go`:

```go
package database

import (
	"database/sql"

	"GoBot/core"
)

// CarrierState stores runtime state for a carrier
type CarrierState struct {
	StationId       string  `db:"station_id"`
	CurrentSystem   *string `db:"current_system"`
	LocationUpdated *int64  `db:"location_updated"`
	JumpTime        *int64  `db:"jump_time"`
	Destination     *string `db:"destination"`
	Status          *string `db:"status"`
}

const carrierSchema = `
CREATE TABLE IF NOT EXISTS carrier_state (
	station_id TEXT PRIMARY KEY,
	current_system TEXT,
	location_updated INTEGER,
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
}
```

**Step 2: Add CRUD functions**

Add to `core/database/carriers.go`:

```go
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
			INSERT INTO carrier_state (station_id, current_system, location_updated, jump_time, destination, status)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT(station_id) DO UPDATE SET
				current_system = excluded.current_system,
				location_updated = excluded.location_updated,
				jump_time = excluded.jump_time,
				destination = excluded.destination,
				status = excluded.status
		`, state.StationId, state.CurrentSystem, state.LocationUpdated, state.JumpTime, state.Destination, state.Status)
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

// UpdateCarrierLocation sets the location and updates the timestamp
func UpdateCarrierLocation(stationId string, system string, timestamp int64) bool {
	state := FetchCarrierState(stationId)
	if state == nil {
		state = &CarrierState{StationId: stationId}
	}
	state.CurrentSystem = &system
	state.LocationUpdated = &timestamp
	return UpsertCarrierState(state)
}
```

**Step 3: Call InitializeCarrierTable from db.go**

In `core/database/db.go`, add at end of `InitalizeDatabase()` (around line 83):

```go
func InitalizeDatabase() {
	db, err := sqlx.Connect("sqlite3", core.Settings.Database())
	if err != nil {
		log.Fatal("Failed to create database", err)
	}

	db.MustExec(schema)
	database = db

	// Initialize carrier table
	InitializeCarrierTable()
}
```

**Step 4: Build to verify**

Run: `go build ./...`
Expected: Build succeeds

**Step 5: Commit**

```bash
git add core/database/carriers.go core/database/db.go
git commit -m "feat: add carrier state database schema

- carrier_state table for runtime data
- CRUD functions for carrier state
- Auto-initialize table on startup"
```

---

## Task 4: Add Inara Location Fetching

**Files:**
- Create: `core/services/inara.go`

**Step 1: Create services directory and inara.go**

Create new file `core/services/inara.go`:

```go
package services

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"GoBot/core"
	"GoBot/core/database"
)

var (
	// Regex to find system name on Inara station page
	// Looking for: <span class="uppercase">System name</span>
	systemRegex = regexp.MustCompile(`(?i)Star system.*?<a[^>]*>([^<]+)</a>`)

	// HTTP client with timeout
	httpClient = &http.Client{Timeout: 10 * time.Second}
)

// FetchInaraLocation fetches carrier location from Inara
func FetchInaraLocation(inaraId int) (string, error) {
	if inaraId <= 0 {
		return "", fmt.Errorf("invalid Inara ID: %d", inaraId)
	}

	url := fmt.Sprintf("https://inara.cz/elite/station/%d/", inaraId)

	resp, err := httpClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch Inara page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Inara returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read Inara response: %w", err)
	}

	matches := systemRegex.FindSubmatch(body)
	if len(matches) < 2 {
		return "", fmt.Errorf("could not find system name on Inara page")
	}

	system := strings.TrimSpace(string(matches[1]))
	return system, nil
}

// GetCarrierLocation gets location from cache or fetches from Inara
func GetCarrierLocation(stationId string, inaraId int) (system string, cached bool, err error) {
	state := database.FetchCarrierState(stationId)
	cacheMinutes := core.Settings.CarrierLocationCacheMinutes()
	now := time.Now().Unix()

	// Check if cache is valid
	if state != nil && state.CurrentSystem != nil && state.LocationUpdated != nil {
		cacheAge := now - *state.LocationUpdated
		if cacheAge < int64(cacheMinutes*60) {
			return *state.CurrentSystem, true, nil
		}
	}

	// Cache expired or doesn't exist - fetch from Inara
	if inaraId <= 0 {
		if state != nil && state.CurrentSystem != nil {
			return *state.CurrentSystem, true, nil
		}
		return "Unknown", false, nil
	}

	system, err = FetchInaraLocation(inaraId)
	if err != nil {
		core.LogErrorF("Failed to fetch Inara location for %s: %s", stationId, err)
		// Return cached value if available
		if state != nil && state.CurrentSystem != nil {
			return *state.CurrentSystem, true, nil
		}
		return "Location unavailable", false, err
	}

	// Update cache
	database.UpdateCarrierLocation(stationId, system, now)
	return system, false, nil
}
```

**Step 2: Build to verify**

Run: `go build ./...`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add core/services/inara.go
git commit -m "feat: add Inara location fetching service

- HTTP fetch and HTML parsing for carrier location
- Caching with configurable expiry
- Graceful fallback to cached data on errors"
```

---

## Task 5: Add Carrier Service Layer

**Files:**
- Create: `core/services/carriers.go`

**Step 1: Create carriers.go service**

Create new file `core/services/carriers.go`:

```go
package services

import (
	"fmt"
	"strings"
	"time"

	"GoBot/core"
	"GoBot/core/database"
)

// CarrierInfo contains full carrier information for display
type CarrierInfo struct {
	StationId     string
	Name          string
	InaraId       int
	CurrentSystem string
	LocationCached bool
	JumpTime      *int64
	Destination   *string
	Status        *string
}

// GetCarrierInfo returns full info for a single carrier
func GetCarrierInfo(stationId string) (*CarrierInfo, error) {
	cfg := core.Settings.GetCarrierByStationId(stationId)
	if cfg == nil {
		return nil, fmt.Errorf("carrier %s not found in config", stationId)
	}

	info := &CarrierInfo{
		StationId: cfg.StationId,
		Name:      cfg.Name,
		InaraId:   cfg.InaraId,
	}

	// Get location
	system, cached, _ := GetCarrierLocation(stationId, cfg.InaraId)
	info.CurrentSystem = system
	info.LocationCached = cached

	// Get runtime state
	state := database.FetchCarrierState(stationId)
	if state != nil {
		info.JumpTime = state.JumpTime
		info.Destination = state.Destination
		info.Status = state.Status
	}

	return info, nil
}

// GetAllCarriersInfo returns info for all configured carriers
func GetAllCarriersInfo() []*CarrierInfo {
	carriers := core.Settings.Carriers()
	result := make([]*CarrierInfo, 0, len(carriers))

	for _, cfg := range carriers {
		info, err := GetCarrierInfo(cfg.StationId)
		if err != nil {
			core.LogErrorF("Failed to get carrier info for %s: %s", cfg.StationId, err)
			continue
		}
		result = append(result, info)
	}

	return result
}

// SetCarrierJumpTime sets the jump time for a carrier
func SetCarrierJumpTime(stationId string, timestamp int64) error {
	if core.Settings.GetCarrierByStationId(stationId) == nil {
		return fmt.Errorf("carrier %s not found", stationId)
	}
	if !database.UpdateCarrierJumpTime(stationId, &timestamp) {
		return fmt.Errorf("failed to update jump time")
	}
	return nil
}

// SetCarrierDestination sets the destination for a carrier
func SetCarrierDestination(stationId string, destination string) error {
	if core.Settings.GetCarrierByStationId(stationId) == nil {
		return fmt.Errorf("carrier %s not found", stationId)
	}
	if !database.UpdateCarrierDestination(stationId, &destination) {
		return fmt.Errorf("failed to update destination")
	}
	return nil
}

// SetCarrierStatus sets the status for a carrier
func SetCarrierStatus(stationId string, status string) error {
	if core.Settings.GetCarrierByStationId(stationId) == nil {
		return fmt.Errorf("carrier %s not found", stationId)
	}
	if !database.UpdateCarrierStatus(stationId, &status) {
		return fmt.Errorf("failed to update status")
	}
	return nil
}

// ClearCarrierField clears specified field(s) for a carrier
func ClearCarrierField(stationId string, field string) error {
	if core.Settings.GetCarrierByStationId(stationId) == nil {
		return fmt.Errorf("carrier %s not found", stationId)
	}

	switch strings.ToLower(field) {
	case "jump":
		database.UpdateCarrierJumpTime(stationId, nil)
	case "dest":
		database.UpdateCarrierDestination(stationId, nil)
	case "status":
		database.UpdateCarrierStatus(stationId, nil)
	case "all":
		database.UpdateCarrierJumpTime(stationId, nil)
		database.UpdateCarrierDestination(stationId, nil)
		database.UpdateCarrierStatus(stationId, nil)
	default:
		return fmt.Errorf("invalid field: %s (use jump, dest, status, or all)", field)
	}
	return nil
}

// FormatCarrierList formats all carriers for display
func FormatCarrierList() string {
	var sb strings.Builder

	sb.WriteString("**OFFICIAL FLEET CARRIERS**\n")
	sb.WriteString("Times shown in your local time\n")
	sb.WriteString("Linked to Inara for up-to-date information\n\n")

	carriers := GetAllCarriersInfo()
	for i, c := range carriers {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(formatSingleCarrier(c))
	}

	return sb.String()
}

func formatSingleCarrier(c *CarrierInfo) string {
	var sb strings.Builder

	// Header: NAME - STATION-ID
	sb.WriteString(fmt.Sprintf("**%s - %s**\n", strings.ToUpper(c.Name), c.StationId))

	// Location
	locationStr := c.CurrentSystem
	if c.LocationCached {
		locationStr += " (cached)"
	}
	sb.WriteString(fmt.Sprintf("\U0001F4CD %s\n", locationStr)) // 📍

	// Jump time
	now := time.Now().Unix()
	if c.JumpTime != nil {
		if *c.JumpTime <= now {
			// Past - departed
			sb.WriteString(fmt.Sprintf("\U0001F680 Departed <t:%d:F> (<t:%d:R>)\n", *c.JumpTime, *c.JumpTime)) // 🚀
		} else {
			// Future - departing
			sb.WriteString(fmt.Sprintf("\u23F1\uFE0F Departing <t:%d:F> (<t:%d:R>)\n", *c.JumpTime, *c.JumpTime)) // ⏱️
		}
	}

	// Destination
	if c.Destination != nil && *c.Destination != "" {
		sb.WriteString(fmt.Sprintf("\U0001F4CC Destination: %s\n", *c.Destination)) // 📌
	}

	// Status or transit message
	if c.JumpTime != nil && *c.JumpTime <= now {
		sb.WriteString("Please remain seated while the carrier is in transit\n")
	} else if c.Status != nil && *c.Status != "" {
		sb.WriteString(fmt.Sprintf("\u2139\uFE0F %s\n", *c.Status)) // ℹ️
	} else if c.JumpTime == nil {
		sb.WriteString("No scheduled jump\n")
	}

	return sb.String()
}

// GetCarrierStationIds returns list of valid station IDs for autocomplete
func GetCarrierStationIds() []string {
	carriers := core.Settings.Carriers()
	ids := make([]string, len(carriers))
	for i, c := range carriers {
		ids[i] = c.StationId
	}
	return ids
}
```

**Step 2: Build to verify**

Run: `go build ./...`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add core/services/carriers.go
git commit -m "feat: add carrier service layer

- GetCarrierInfo, GetAllCarriersInfo
- SetCarrierJumpTime, SetCarrierDestination, SetCarrierStatus
- ClearCarrierField with jump/dest/status/all options
- FormatCarrierList for Discord display"
```

---

## Task 6: Add Message Command Handler

**Files:**
- Create: `core/dispatch/handlers/carriers.go`

**Step 1: Create carriers.go handler**

Create new file `core/dispatch/handlers/carriers.go`:

```go
package handlers

import (
	"strconv"
	"strings"

	"GoBot/core"
	"GoBot/core/dispatch"
	"GoBot/core/services"
)

type carriers struct {
	dispatch.NoOpMessageHandler
}

const (
	CarrierJump   = "carrierjump"
	CarrierDest   = "carrierdest"
	CarrierStatus = "carrierstatus"
	CarrierClear  = "carrierclear"
	CarriersList  = "carriers"
)

func (*carriers) CommandGroup() string {
	return "Fleet Carriers"
}

func init() {
	dispatch.Register(&carriers{},
		[]dispatch.MessageCommand{
			{CarrierJump, "Set carrier jump time. Arguments: *<station-id> <unix-timestamp>*"},
			{CarrierDest, "Set carrier destination. Arguments: *<station-id> <system name>*"},
			{CarrierStatus, "Set carrier status. Arguments: *<station-id> <status text>*"},
			{CarrierClear, "Clear carrier field. Arguments: *<station-id> <jump|dest|status|all>*"},
			{CarriersList, "List all fleet carriers with current status."},
		},
		nil, false)
}

func (c *carriers) HandleCommand(m *dispatch.Message) bool {
	switch m.Command {
	case CarriersList:
		handleCarriersList(m)
		return true
	case CarrierJump, CarrierDest, CarrierStatus, CarrierClear:
		return handleCarrierManagement(m)
	default:
		return false
	}
}

// canManageCarriers checks if user has permission to manage carriers
func canManageCarriers(m *dispatch.Message) bool {
	// Secure channel always allowed
	if m.ChannelID == SecureChannnel {
		return true
	}
	// Check if user is a carrier owner
	return core.Settings.IsCarrierOwner(m.Author.ID)
}

func handleCarrierManagement(m *dispatch.Message) bool {
	if !canManageCarriers(m) {
		m.ReplyToChannel("You don't have permission to manage carriers.")
		return true
	}

	if len(m.Args) < 1 {
		m.ReplyToChannel("**Error:** Missing station ID. Usage: `%s%s <station-id> ...`",
			core.Settings.CommandPrefix(), m.Command)
		return true
	}

	stationId := strings.ToUpper(m.Args[0])

	// Validate station ID exists
	if core.Settings.GetCarrierByStationId(stationId) == nil {
		validIds := services.GetCarrierStationIds()
		m.ReplyToChannel("**Error:** Carrier `%s` not found. Valid carriers: %s",
			stationId, strings.Join(validIds, ", "))
		return true
	}

	switch m.Command {
	case CarrierJump:
		handleSetJumpTime(m, stationId)
	case CarrierDest:
		handleSetDestination(m, stationId)
	case CarrierStatus:
		handleSetStatus(m, stationId)
	case CarrierClear:
		handleClearField(m, stationId)
	}
	return true
}

func handleSetJumpTime(m *dispatch.Message, stationId string) {
	if len(m.Args) < 2 {
		m.ReplyToChannel("**Error:** Missing timestamp. Usage: `%s%s %s <unix-timestamp>`",
			core.Settings.CommandPrefix(), CarrierJump, stationId)
		return
	}

	timestamp, err := strconv.ParseInt(m.Args[1], 10, 64)
	if err != nil {
		m.ReplyToChannel("**Error:** Invalid timestamp `%s`. Expected unix timestamp (e.g., 1737399000)", m.Args[1])
		return
	}

	if err := services.SetCarrierJumpTime(stationId, timestamp); err != nil {
		m.ReplyToChannel("**Error:** %s", err)
		return
	}

	m.ReplyToChannel("Jump time for **%s** set to <t:%d:F> (<t:%d:R>)", stationId, timestamp, timestamp)
}

func handleSetDestination(m *dispatch.Message, stationId string) {
	if len(m.Args) < 2 {
		m.ReplyToChannel("**Error:** Missing destination. Usage: `%s%s %s <system name>`",
			core.Settings.CommandPrefix(), CarrierDest, stationId)
		return
	}

	destination := strings.Join(m.Args[1:], " ")
	if err := services.SetCarrierDestination(stationId, destination); err != nil {
		m.ReplyToChannel("**Error:** %s", err)
		return
	}

	m.ReplyToChannel("Destination for **%s** set to **%s**", stationId, destination)
}

func handleSetStatus(m *dispatch.Message, stationId string) {
	if len(m.Args) < 2 {
		m.ReplyToChannel("**Error:** Missing status. Usage: `%s%s %s <status text>`",
			core.Settings.CommandPrefix(), CarrierStatus, stationId)
		return
	}

	status := strings.Join(m.Args[1:], " ")
	if err := services.SetCarrierStatus(stationId, status); err != nil {
		m.ReplyToChannel("**Error:** %s", err)
		return
	}

	m.ReplyToChannel("Status for **%s** set to: %s", stationId, status)
}

func handleClearField(m *dispatch.Message, stationId string) {
	if len(m.Args) < 2 {
		m.ReplyToChannel("**Error:** Missing field. Usage: `%s%s %s <jump|dest|status|all>`",
			core.Settings.CommandPrefix(), CarrierClear, stationId)
		return
	}

	field := strings.ToLower(m.Args[1])
	if err := services.ClearCarrierField(stationId, field); err != nil {
		m.ReplyToChannel("**Error:** %s", err)
		return
	}

	if field == "all" {
		m.ReplyToChannel("All fields cleared for **%s**", stationId)
	} else {
		m.ReplyToChannel("Field `%s` cleared for **%s**", field, stationId)
	}
}

func handleCarriersList(m *dispatch.Message) {
	output := services.FormatCarrierList()

	// Reply in channel if bot channel, otherwise DM
	if core.Settings.IsBotChannel(m.ChannelID) {
		m.ReplyToChannel("%s", output)
	} else {
		m.ReplyToSender("%s", output)
	}
}
```

**Step 2: Build to verify**

Run: `go build ./...`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add core/dispatch/handlers/carriers.go
git commit -m "feat: add carrier message command handlers

- carrierjump, carrierdest, carrierstatus, carrierclear commands
- carriers list command with DM/channel logic
- Permission checks for carrier commanders"
```

---

## Task 7: Add Slash Command Handler

**Files:**
- Create: `core/dispatch/handlers/carriers_slash.go`
- Modify: `main.go`

**Step 1: Create slash command handler**

Create new file `core/dispatch/handlers/carriers_slash.go`:

```go
package handlers

import (
	"strings"

	"GoBot/core"
	"GoBot/core/services"
	"github.com/bwmarrin/discordgo"
)

var carrierSlashCommands = []*discordgo.ApplicationCommand{
	{
		Name:        "carriers",
		Description: "List all fleet carriers with current status",
	},
	{
		Name:        "carrierjump",
		Description: "Set carrier jump time",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "carrier",
				Description:  "Carrier station ID",
				Required:     true,
				Autocomplete: true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "timestamp",
				Description: "Unix timestamp for jump time",
				Required:    true,
			},
		},
	},
	{
		Name:        "carrierdest",
		Description: "Set carrier destination",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "carrier",
				Description:  "Carrier station ID",
				Required:     true,
				Autocomplete: true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "system",
				Description: "Destination system name",
				Required:    true,
			},
		},
	},
	{
		Name:        "carrierstatus",
		Description: "Set carrier status message",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "carrier",
				Description:  "Carrier station ID",
				Required:     true,
				Autocomplete: true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "status",
				Description: "Status message",
				Required:    true,
			},
		},
	},
	{
		Name:        "carrierclear",
		Description: "Clear carrier field",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "carrier",
				Description:  "Carrier station ID",
				Required:     true,
				Autocomplete: true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "field",
				Description: "Field to clear",
				Required:    true,
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{Name: "Jump Time", Value: "jump"},
					{Name: "Destination", Value: "dest"},
					{Name: "Status", Value: "status"},
					{Name: "All", Value: "all"},
				},
			},
		},
	},
}

// RegisterCarrierSlashCommands registers slash commands with Discord
func RegisterCarrierSlashCommands(s *discordgo.Session) {
	for _, cmd := range carrierSlashCommands {
		_, err := s.ApplicationCommandCreate(s.State.User.ID, "", cmd)
		if err != nil {
			core.LogErrorF("Failed to register slash command %s: %s", cmd.Name, err)
		} else {
			core.LogInfoF("Registered slash command: /%s", cmd.Name)
		}
	}
}

// HandleCarrierSlashCommand handles carrier slash command interactions
func HandleCarrierSlashCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		handleCarrierCommand(s, i)
	case discordgo.InteractionApplicationCommandAutocomplete:
		handleCarrierAutocomplete(s, i)
	}
}

func handleCarrierCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()

	// Get user ID (works for both guild and DM)
	var userID string
	if i.Member != nil {
		userID = i.Member.User.ID
	} else if i.User != nil {
		userID = i.User.ID
	}

	switch data.Name {
	case "carriers":
		output := services.FormatCarrierList()
		respond(s, i, output, true)

	case "carrierjump":
		if !canManageCarriersSlash(userID, i.ChannelID) {
			respond(s, i, "You don't have permission to manage carriers.", true)
			return
		}
		stationId := strings.ToUpper(data.Options[0].StringValue())
		timestamp := data.Options[1].IntValue()

		if err := services.SetCarrierJumpTime(stationId, timestamp); err != nil {
			respond(s, i, "**Error:** "+err.Error(), true)
			return
		}
		respond(s, i, formatJumpTimeResponse(stationId, timestamp), true)

	case "carrierdest":
		if !canManageCarriersSlash(userID, i.ChannelID) {
			respond(s, i, "You don't have permission to manage carriers.", true)
			return
		}
		stationId := strings.ToUpper(data.Options[0].StringValue())
		destination := data.Options[1].StringValue()

		if err := services.SetCarrierDestination(stationId, destination); err != nil {
			respond(s, i, "**Error:** "+err.Error(), true)
			return
		}
		respond(s, i, formatDestinationResponse(stationId, destination), true)

	case "carrierstatus":
		if !canManageCarriersSlash(userID, i.ChannelID) {
			respond(s, i, "You don't have permission to manage carriers.", true)
			return
		}
		stationId := strings.ToUpper(data.Options[0].StringValue())
		status := data.Options[1].StringValue()

		if err := services.SetCarrierStatus(stationId, status); err != nil {
			respond(s, i, "**Error:** "+err.Error(), true)
			return
		}
		respond(s, i, formatStatusResponse(stationId, status), true)

	case "carrierclear":
		if !canManageCarriersSlash(userID, i.ChannelID) {
			respond(s, i, "You don't have permission to manage carriers.", true)
			return
		}
		stationId := strings.ToUpper(data.Options[0].StringValue())
		field := data.Options[1].StringValue()

		if err := services.ClearCarrierField(stationId, field); err != nil {
			respond(s, i, "**Error:** "+err.Error(), true)
			return
		}
		respond(s, i, formatClearResponse(stationId, field), true)
	}
}

func handleCarrierAutocomplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()

	var choices []*discordgo.ApplicationCommandOptionChoice
	carriers := core.Settings.Carriers()

	for _, c := range carriers {
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  c.Name + " (" + c.StationId + ")",
			Value: c.StationId,
		})
	}

	// Filter based on what user has typed
	for _, opt := range data.Options {
		if opt.Focused {
			typed := strings.ToLower(opt.StringValue())
			if typed != "" {
				filtered := make([]*discordgo.ApplicationCommandOptionChoice, 0)
				for _, c := range choices {
					if strings.Contains(strings.ToLower(c.Name), typed) ||
						strings.Contains(strings.ToLower(c.Value.(string)), typed) {
						filtered = append(filtered, c)
					}
				}
				choices = filtered
			}
			break
		}
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Choices: choices,
		},
	})
}

func canManageCarriersSlash(userID, channelID string) bool {
	if channelID == SecureChannnel {
		return true
	}
	return core.Settings.IsCarrierOwner(userID)
}

func respond(s *discordgo.Session, i *discordgo.InteractionCreate, content string, ephemeral bool) {
	flags := discordgo.MessageFlags(0)
	if ephemeral {
		flags = discordgo.MessageFlagsEphemeral
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   flags,
		},
	})
}

func formatJumpTimeResponse(stationId string, timestamp int64) string {
	return "Jump time for **" + stationId + "** set to <t:" +
		formatInt(timestamp) + ":F> (<t:" + formatInt(timestamp) + ":R>)"
}

func formatDestinationResponse(stationId, destination string) string {
	return "Destination for **" + stationId + "** set to **" + destination + "**"
}

func formatStatusResponse(stationId, status string) string {
	return "Status for **" + stationId + "** set to: " + status
}

func formatClearResponse(stationId, field string) string {
	if field == "all" {
		return "All fields cleared for **" + stationId + "**"
	}
	return "Field `" + field + "` cleared for **" + stationId + "**"
}

func formatInt(n int64) string {
	return strings.TrimSpace(strings.Replace(
		strings.Replace(
			strings.Replace(
				string(rune(n)), "", "", -1), "", "", -1), "", "", -1))
}
```

**Step 2: Fix formatInt helper**

Replace the formatInt function with proper implementation:

```go
import "strconv"

func formatInt(n int64) string {
	return strconv.FormatInt(n, 10)
}
```

**Step 3: Update main.go to register slash commands**

Modify `main.go` to add interaction handler and register commands:

```go
package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"GoBot/core"
	"GoBot/core/database"
	_ "GoBot/core/database"
	"GoBot/core/dispatch"
	"GoBot/core/dispatch/handlers"
	_ "GoBot/core/dispatch/handlers"

	"github.com/bwmarrin/discordgo"
)

var (
	settingsFile string
)

func init() {
	flag.StringVar(&settingsFile, "c", "config-dev.json", "Configuration path")
	flag.Parse()
}

func main() {
	core.LoadSettings(settingsFile)
	database.InitalizeDatabase()
	defer database.Close()
	dispatch.SettingsLoaded()

	dg, err := discordgo.New("Bot " + core.Settings.AuthToken())
	if err != nil {
		core.LogFatal("error creating Discord session,", err)
		return
	}

	// Register handlers
	dg.AddHandler(messageCreate)
	dg.AddHandler(messageUpdate)
	dg.AddHandler(interactionCreate)

	err = dg.Open()
	if err != nil {
		core.LogFatal("error opening connection,", err)
		return
	}
	defer dg.Close()

	// Register slash commands after connection is open
	handlers.RegisterCarrierSlashCommands(dg)

	core.LogInfoF("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	go dispatch.Dispatch(s, m.Message)
}

func messageUpdate(s *discordgo.Session, m *discordgo.MessageUpdate) {
	go dispatch.Dispatch(s, m.Message)
}

func interactionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	handlers.HandleCarrierSlashCommand(s, i)
}
```

**Step 4: Build to verify**

Run: `go build ./...`
Expected: Build succeeds

**Step 5: Commit**

```bash
git add core/dispatch/handlers/carriers_slash.go main.go
git commit -m "feat: add carrier slash commands

- /carriers, /carrierjump, /carrierdest, /carrierstatus, /carrierclear
- Autocomplete for carrier selection
- Ephemeral responses for all commands
- Register commands on bot startup"
```

---

## Task 8: Final Testing and Cleanup

**Step 1: Full build and vet**

Run:
```bash
go build ./...
go vet ./...
```

Expected: No errors

**Step 2: Update config-dev.json with test carriers**

Add carrier configuration (user will provide Inara IDs):

```json
{
  "carrierOwnerIds": [],
  "carriers": [
    {"stationId": "W7H-6DZ", "inaraId": 0, "name": "DSEV Odysseus"},
    {"stationId": "V2W-85Z", "inaraId": 0, "name": "DSEV Distant Suns"},
    {"stationId": "V4V-2XZ", "inaraId": 0, "name": "DSEC Fimbulthul"},
    {"stationId": "TBQ-6VX", "inaraId": 0, "name": "Pillar of Chista"}
  ],
  "carrierLocationCacheMinutes": 20
}
```

**Step 3: Commit final changes**

```bash
git add -A
git commit -m "chore: finalize carrier commands implementation"
```

---

## Summary

| Task | Description | Files |
|------|-------------|-------|
| 1 | Rename config field | settings.go, custom.go, config.json.example |
| 2 | Add carrier config | settings.go, config.json.example |
| 3 | Database schema | carriers.go, db.go |
| 4 | Inara fetching | services/inara.go |
| 5 | Carrier service | services/carriers.go |
| 6 | Message commands | handlers/carriers.go |
| 7 | Slash commands | handlers/carriers_slash.go, main.go |
| 8 | Testing/cleanup | config-dev.json |
