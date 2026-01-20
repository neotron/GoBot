package services

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/json"
	"io"
	"strings"
	"time"

	"GoBot/core"
	"GoBot/core/database"

	"github.com/go-zeromq/zmq4"
)

const (
	eddnRelayURL = "tcp://eddn.edcd.io:9500"
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
		processJournalMessage(eddnMsg.Message)
	case strings.Contains(eddnMsg.Schema, "/fscarrierstate/"):
		processFSCarrierState(eddnMsg.Message)
	}
}

func processJournalMessage(raw json.RawMessage) {
	var msg JournalMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return
	}

	// Check for carrier-related events
	switch msg.Event {
	case "CarrierJump":
		// CarrierJump includes the carrier ID and destination - carrier has jumped
		if msg.CarrierID != "" && carrierCallsigns[msg.CarrierID] {
			updateCarrierFromEDDN(msg.CarrierID, msg.StarSystem, msg.Timestamp, msg.Event)
			// Clear pending jump since the jump completed
			database.ClearCarrierPendingJump(msg.CarrierID)
		}
	case "CarrierJumpRequest":
		// Carrier jump has been scheduled
		if msg.CarrierID != "" && carrierCallsigns[msg.CarrierID] {
			updateCarrierPendingJump(msg.CarrierID, msg.StarSystem, msg.DepartureTime)
		}
	case "CarrierJumpCancelled":
		// Carrier jump has been cancelled
		if msg.CarrierID != "" && carrierCallsigns[msg.CarrierID] {
			clearCarrierPendingJump(msg.CarrierID)
		}
	case "Location", "FSDJump":
		// If the station name matches a carrier callsign format (XXX-XXX)
		if isCarrierCallsign(msg.StationName) && carrierCallsigns[msg.StationName] {
			updateCarrierFromEDDN(msg.StationName, msg.StarSystem, msg.Timestamp, msg.Event)
		}
	case "Docked":
		// Docked at a carrier
		if isCarrierCallsign(msg.StationName) && carrierCallsigns[msg.StationName] {
			updateCarrierFromEDDN(msg.StationName, msg.StarSystem, msg.Timestamp, msg.Event)
		}
	}
}

func processFSCarrierState(raw json.RawMessage) {
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
		updateCarrierFromEDDN(callsign, msg.StarSystem, msg.Timestamp, msg.Event)
	}
}

func updateCarrierFromEDDN(stationId, system, timestamp, eventType string) {
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

	// Sanity check: reject timestamps more than 1 minute in the future
	now := time.Now().Unix()
	if eventTime > now+60 {
		core.LogDebugF("EDDN: Skipping future %s for %s at %s (event: %d, now: %d)", eventType, stationId, eventTimeStr, eventTime, now)
		return
	}

	// Check if this event is newer than what we have
	state := database.FetchCarrierState(stationId)
	if state != nil && state.LocationUpdated != nil && *state.LocationUpdated >= eventTime {
		// We already have a newer or same-time update, skip
		core.LogDebugF("EDDN: Skipping old %s for %s at %s (have: %d)", eventType, stationId, eventTimeStr, *state.LocationUpdated)
		return
	}

	_, changed := database.UpdateCarrierLocation(stationId, system, "", eventTime)

	if changed {
		core.LogInfoF("EDDN: %s - Carrier %s location changed to %s at %s", eventType, stationId, system, eventTimeStr)
		PostCarrierFlightLog(stationId, []string{"location: " + system})
	} else {
		core.LogDebugF("EDDN: %s - Carrier %s location confirmed at %s (%s)", eventType, stationId, system, eventTimeStr)
	}
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
		core.LogDebugF("EDDN: Failed to parse departure time '%s' for %s", departureTime, stationId)
		return
	}

	database.UpdateCarrierPendingJump(stationId, &destSystem, &jumpTime)
	core.LogInfoF("EDDN: CarrierJumpRequest - Carrier %s scheduled jump to %s at %s", stationId, destSystem, departureTime)
	PostCarrierFlightLog(stationId, []string{"pending jump: " + destSystem})
}

func clearCarrierPendingJump(stationId string) {
	database.ClearCarrierPendingJump(stationId)
	core.LogInfoF("EDDN: CarrierJumpCancelled - Carrier %s jump cancelled", stationId)
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
