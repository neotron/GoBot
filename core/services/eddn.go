package services

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"GoBot/core"
	"GoBot/core/database"

	"github.com/go-zeromq/zmq4"
)

const (
	eddnRelayURL              = "tcp://eddn.edcd.io:9500"
	carrierJumpCooldown       = 20 * 60 // 20 minutes in seconds
	suspiciousDistanceThreshold = 500.0 // ly - max expected single jump distance
)

// EDDNMessage is the outer wrapper for all EDDN messages
type EDDNMessage struct {
	Schema  string          `json:"$schemaRef"`
	Header  EDDNHeader      `json:"header"`
	Message json.RawMessage `json:"message"`
}

// EDDNHeader contains metadata about the message
type EDDNHeader struct {
	UploaderID      string `json:"uploaderID"`
	SoftwareName    string `json:"softwareName"`
	SoftwareVersion string `json:"softwareVersion"`
	GatewayTimestamp string `json:"gatewayTimestamp"`
}

// JournalMessage represents a journal event from EDDN
type JournalMessage struct {
	Timestamp      string `json:"timestamp"` // ISO 8601 timestamp of the event
	Event          string `json:"event"`
	StarSystem     string `json:"StarSystem"`
	SystemAddress  int64  `json:"SystemAddress"`
	StationName    string `json:"StationName"`
	MarketID       int64  `json:"MarketID"`
	// For CarrierJump events
	Docked    bool   `json:"Docked"`
	CarrierID string `json:"CarrierID,omitempty"`
	// For CarrierJumpRequest events
	DepartureTime string `json:"DepartureTime,omitempty"` // ISO 8601 scheduled departure
}

// FSCarrierState represents the FSCarrierState event
type FSCarrierState struct {
	Timestamp     string `json:"timestamp"` // ISO 8601 timestamp of the event
	Event         string `json:"event"`
	CarrierID     string `json:"CarrierID"`
	Callsign      string `json:"Callsign"`
	StarSystem    string `json:"StarSystem"`
	SystemAddress int64  `json:"SystemAddress"`
}

// carrierCallsigns maps our carrier station IDs for quick lookup
var carrierCallsigns map[string]bool

// suspiciousLocation tracks unvalidated location updates
type suspiciousLocation struct {
	System      string
	EventTime   int64
	FirstSeen   int64
	Validations int // number of independent confirmations
}

// suspiciousLocations maps stationId -> pending suspicious location
var suspiciousLocations = make(map[string]*suspiciousLocation)

// StartEDDNListener starts the EDDN listener in a goroutine
func StartEDDNListener() {
	// Build lookup map of our carrier callsigns
	carrierCallsigns = make(map[string]bool)
	for _, c := range core.Settings.Carriers() {
		carrierCallsigns[c.StationId] = true
	}

	if len(carrierCallsigns) == 0 {
		core.LogInfo("No carriers configured, EDDN listener not started")
		return
	}

	go eddnListenerLoop()
}

func eddnListenerLoop() {
	for {
		err := connectAndListen()
		if err != nil {
			core.LogErrorF("EDDN listener error: %s, reconnecting in 30s...", err)
			time.Sleep(30 * time.Second)
		}
	}
}

func connectAndListen() error {
	ctx := context.Background()

	sub := zmq4.NewSub(ctx)
	defer sub.Close()

	err := sub.Dial(eddnRelayURL)
	if err != nil {
		return err
	}

	// Subscribe to all messages
	err = sub.SetOption(zmq4.OptionSubscribe, "")
	if err != nil {
		return err
	}

	core.LogInfo("EDDN listener connected to ", eddnRelayURL)

	for {
		msg, err := sub.Recv()
		if err != nil {
			return err
		}

		if len(msg.Frames) == 0 {
			continue
		}

		go processEDDNMessage(msg.Frames[0])
	}
}

func processEDDNMessage(compressed []byte) {
	// Decompress zlib
	reader, err := zlib.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return // Silently ignore malformed messages
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return
	}

	// Parse outer message
	var eddnMsg EDDNMessage
	if err := json.Unmarshal(data, &eddnMsg); err != nil {
		return
	}

	// Route based on schema
	switch {
	case strings.Contains(eddnMsg.Schema, "/journal/"):
		processJournalMessage(eddnMsg.Message, eddnMsg.Header.UploaderID)
	case strings.Contains(eddnMsg.Schema, "/fscarrierstate/"):
		processFSCarrierState(eddnMsg.Message, eddnMsg.Header.UploaderID)
	}
}

func processJournalMessage(raw json.RawMessage, uploaderID string) {
	var msg JournalMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return
	}

	// Check for carrier-related events
	switch msg.Event {
	case "CarrierJump":
		// CarrierJump includes the carrier ID and destination - carrier has jumped
		if msg.CarrierID != "" && carrierCallsigns[msg.CarrierID] {
			updateCarrierFromEDDN(msg.CarrierID, msg.StarSystem, msg.Timestamp, msg.Event, uploaderID)
			// Clear pending jump since the jump completed
			database.ClearCarrierPendingJump(msg.CarrierID)
		}
	case "CarrierJumpRequest":
		// Carrier jump has been scheduled
		if msg.CarrierID != "" {
			core.LogDebugF("EDDN CarrierJumpRequest: %s -> %s (departure: %s) [%s]", msg.CarrierID, msg.StarSystem, msg.DepartureTime, uploaderID)
			if carrierCallsigns[msg.CarrierID] {
				updateCarrierPendingJump(msg.CarrierID, msg.StarSystem, msg.DepartureTime)
			}
		}
	case "CarrierJumpCancelled":
		// Carrier jump has been cancelled
		if msg.CarrierID != "" {
			core.LogDebugF("EDDN CarrierJumpCancelled: %s [%s]", msg.CarrierID, uploaderID)
			if carrierCallsigns[msg.CarrierID] {
				clearCarrierPendingJump(msg.CarrierID)
			}
		}
	case "Location", "FSDJump":
		// If the station name matches a carrier callsign format (XXX-XXX)
		if isCarrierCallsign(msg.StationName) && carrierCallsigns[msg.StationName] {
			updateCarrierFromEDDN(msg.StationName, msg.StarSystem, msg.Timestamp, msg.Event, uploaderID)
		}
	case "Docked":
		// Docked at a carrier
		if isCarrierCallsign(msg.StationName) && carrierCallsigns[msg.StationName] {
			updateCarrierFromEDDN(msg.StationName, msg.StarSystem, msg.Timestamp, msg.Event, uploaderID)
		}
	}

	// Check for follower carriers in Location/FSDJump/Docked events
	if (msg.Event == "Location" || msg.Event == "FSDJump" || msg.Event == "Docked") &&
		msg.StationName != "" && isCarrierCallsign(msg.StationName) && !isOurCarrier(msg.StationName) {
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
}

func processFSCarrierState(raw json.RawMessage, uploaderID string) {
	var msg FSCarrierState
	if err := json.Unmarshal(raw, &msg); err != nil {
		return
	}

	// Check if this is one of our carriers
	callsign := msg.Callsign
	if callsign == "" {
		callsign = msg.CarrierID
	}

	if carrierCallsigns[callsign] {
		updateCarrierFromEDDN(callsign, msg.StarSystem, msg.Timestamp, msg.Event, uploaderID)
	}
}

func updateCarrierFromEDDN(stationId, system, timestamp, eventType, uploaderID string) {
	if system == "" {
		return
	}

	// Parse ISO 8601 timestamp from EDDN, fall back to current time
	var eventTime int64
	var eventTimeStr string
	if t, err := time.Parse(time.RFC3339, timestamp); err == nil {
		eventTime = t.Unix()
		eventTimeStr = t.Format("2006-01-02 15:04:05")
	} else if t, err := time.Parse("2006-01-02T15:04:05Z", timestamp); err == nil {
		eventTime = t.Unix()
		eventTimeStr = t.Format("2006-01-02 15:04:05")
	} else {
		eventTime = time.Now().Unix()
		eventTimeStr = time.Now().Format("2006-01-02 15:04:05")
	}

	now := time.Now().Unix()

	// Sanity check: reject timestamps more than 1 minute in the future
	if eventTime > now+60 {
		core.LogDebugF("EDDN: Skipping future %s for %s at %s (event: %d, now: %d) [%s]", eventType, getCarrierDisplayName(stationId), eventTimeStr, eventTime, now, uploaderID)
		return
	}

	// Check if this event is newer than what we have
	state := database.FetchCarrierState(stationId)
	if state != nil && state.LocationUpdated != nil && *state.LocationUpdated >= eventTime {
		// We already have a newer or same-time update, skip
		core.LogDebugF("EDDN: Skipping old %s for %s at %s (have: %d) [%s]", eventType, getCarrierDisplayName(stationId), eventTimeStr, *state.LocationUpdated, uploaderID)
		return
	}

	// Determine if this update is suspicious and needs validation
	suspicious, reason := isLocationSuspicious(stationId, system, state, eventTime)
	if suspicious {
		handleSuspiciousLocation(stationId, system, eventTime, eventType, uploaderID, reason)
		return
	}

	// Clear any pending suspicious location since we're applying a valid update
	delete(suspiciousLocations, stationId)

	_, changed := database.UpdateCarrierLocation(stationId, system, "", eventTime)

	if changed {
		core.LogInfoF("EDDN: %s - %s location changed to %s at %s [%s]", eventType, getCarrierDisplayName(stationId), system, eventTimeStr, uploaderID)
		PostCarrierFlightLog(stationId, []string{"location: " + system})
	} else {
		core.LogDebugF("EDDN: %s - %s location confirmed at %s (%s) [%s]", eventType, getCarrierDisplayName(stationId), system, eventTimeStr, uploaderID)
	}
}

// isLocationSuspicious checks if a location update should require validation
func isLocationSuspicious(stationId, newSystem string, state *database.CarrierState, eventTime int64) (bool, string) {
	// Check 1: Is the system known in EDSM? (always checked)
	coords, err := GetSystemCoords(newSystem)
	if err != nil || coords == nil {
		return true, "unknown system"
	}

	if state == nil || state.CurrentSystem == nil {
		return false, "" // First location, accept it
	}

	currentSystem := *state.CurrentSystem
	if currentSystem == "" || currentSystem == "Unknown" {
		return false, "" // No previous location, accept it
	}

	// If new system matches current system, this is just a confirmation, not suspicious
	if strings.EqualFold(newSystem, currentSystem) {
		return false, ""
	}

	// From here on, the location is actually changing

	// Check 2: Is the distance reasonable? (< 500ly) - only if "range" validation enabled
	if core.Settings.CarrierValidationEnabled("range") {
		currentCoords, err := GetSystemCoords(currentSystem)
		if err == nil && currentCoords != nil {
			dist := CalculateDistance(currentCoords, coords)
			if dist > suspiciousDistanceThreshold {
				return true, "distance too far (" + formatDistance(dist) + " ly)"
			}
		}
	}

	// Check 3: Has enough time passed since last location change? (20 min cooldown) - only if "time" validation enabled
	if core.Settings.CarrierValidationEnabled("time") {
		if state.LocationChanged != nil {
			timeSinceChange := eventTime - *state.LocationChanged
			if timeSinceChange < carrierJumpCooldown {
				return true, "too soon since last jump (" + formatDuration(timeSinceChange) + ")"
			}
		}
	}

	return false, ""
}

// handleSuspiciousLocation tracks suspicious location and applies if validated
func handleSuspiciousLocation(stationId, system string, eventTime int64, eventType, uploaderID, reason string) {
	now := time.Now().Unix()

	pending := suspiciousLocations[stationId]
	if pending != nil && pending.System == system {
		// Same location reported again - count as validation
		pending.Validations++
		core.LogDebugF("EDDN: Suspicious %s for %s -> %s validated (%d/2) [%s] reason: %s",
			eventType, getCarrierDisplayName(stationId), system, pending.Validations, uploaderID, reason)

		if pending.Validations >= 2 {
			// Enough validations, apply the update
			core.LogInfoF("EDDN: %s - %s location validated and changed to %s [%s]", eventType, getCarrierDisplayName(stationId), system, uploaderID)
			database.UpdateCarrierLocation(stationId, system, "", eventTime)
			PostCarrierFlightLog(stationId, []string{"location: " + system + " (validated)"})
			delete(suspiciousLocations, stationId)
		}
		return
	}

	// New suspicious location or different system - start tracking
	suspiciousLocations[stationId] = &suspiciousLocation{
		System:      system,
		EventTime:   eventTime,
		FirstSeen:   now,
		Validations: 1,
	}
	core.LogWarnF("EDDN: Suspicious %s for %s -> %s, needs validation (1/2) [%s] reason: %s",
		eventType, getCarrierDisplayName(stationId), system, uploaderID, reason)
}

func formatDistance(d float64) string {
	return fmt.Sprintf("%.1f", d)
}

func formatDuration(seconds int64) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	return fmt.Sprintf("%dm", seconds/60)
}

func updateCarrierPendingJump(stationId, destSystem, departureTime string) {
	if destSystem == "" || departureTime == "" {
		return
	}

	// Parse departure time
	var jumpTime int64
	if t, err := time.Parse(time.RFC3339, departureTime); err == nil {
		jumpTime = t.Unix()
	} else if t, err := time.Parse("2006-01-02T15:04:05Z", departureTime); err == nil {
		jumpTime = t.Unix()
	} else {
		core.LogDebugF("EDDN: Failed to parse departure time '%s' for %s", departureTime, getCarrierDisplayName(stationId))
		return
	}

	database.UpdateCarrierPendingJump(stationId, &destSystem, &jumpTime)
	core.LogInfoF("EDDN: CarrierJumpRequest - %s scheduled jump to %s at %s", getCarrierDisplayName(stationId), destSystem, departureTime)
	PostCarrierFlightLog(stationId, []string{"pending jump: " + destSystem})
}

func clearCarrierPendingJump(stationId string) {
	database.ClearCarrierPendingJump(stationId)
	core.LogInfoF("EDDN: CarrierJumpCancelled - %s jump cancelled", getCarrierDisplayName(stationId))
	PostCarrierFlightLog(stationId, []string{"jump cancelled"})
}

// isCarrierCallsign checks if a string looks like a carrier callsign (XXX-XXX)
func isCarrierCallsign(s string) bool {
	if len(s) != 7 {
		return false
	}
	if s[3] != '-' {
		return false
	}
	// Check alphanumeric parts
	for i, c := range s {
		if i == 3 {
			continue
		}
		if !((c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
			return false
		}
	}
	return true
}

// isOurCarrier checks if a station ID is one of our configured carriers
func isOurCarrier(stationId string) bool {
	return carrierCallsigns[stationId]
}

// getCarrierDisplayName returns "Name (CALLSIGN)" for our carriers, or just the callsign otherwise
func getCarrierDisplayName(stationId string) string {
	cfg := core.Settings.GetCarrierByStationId(stationId)
	if cfg != nil {
		return fmt.Sprintf("%s (%s)", cfg.Name, stationId)
	}
	return stationId
}

// checkAndRecordFollower checks if an external carrier is near any of our carriers
func checkAndRecordFollower(followerStationId, system string, eventTime int64) {
	// Don't track our own carriers as followers
	if isOurCarrier(followerStationId) {
		return
	}

	core.LogTraceF("EDDN: External carrier %s seen at %s", followerStationId, system)

	threshold := core.Settings.FollowerDistanceThreshold()

	// Get coordinates for the follower's system
	followerCoords, err := GetSystemCoords(system)
	if err != nil || followerCoords == nil {
		core.LogTraceF("EDDN: External carrier %s - cannot get coords for %s", followerStationId, system)
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
					followerStationId, getCarrierDisplayName(stationId), system, distance)
			} else {
				core.LogTraceF("EDDN: Follower %s updated near %s at %s (%.1f ly, same location)",
					followerStationId, getCarrierDisplayName(stationId), system, distance)
			}
			return // Only record once per event (nearest carrier)
		}
		core.LogTraceF("EDDN: External carrier %s at %s is %.1f ly from %s (threshold: %.0f)",
			followerStationId, system, distance, getCarrierDisplayName(stationId), threshold)
	}
}
