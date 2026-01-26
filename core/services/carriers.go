package services

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"GoBot/core"
	"GoBot/core/database"

	"github.com/bwmarrin/discordgo"
)

var discordSession *discordgo.Session

// SetDiscordSession sets the Discord session for posting flight logs
func SetDiscordSession(s *discordgo.Session) {
	discordSession = s
}

// CarrierInfo contains full carrier information for display
type CarrierInfo struct {
	StationId       string
	Name            string
	InaraId         int
	CurrentSystem   string
	LocationUpdated *int64  // Last time we received a location update
	LocationChanged *int64  // Last time location actually changed
	JumpTime        *int64  // Manual jump time
	Destination     *string // Manual destination
	Status          *string
	PendingJumpDest *string // EDDN: scheduled jump destination
	PendingJumpTime *int64  // EDDN: scheduled jump departure time
}

// GetCarrierInfo returns full info for a single carrier
func GetCarrierInfo(stationId string) (*CarrierInfo, error) {
	cfg := core.Settings.GetCarrierByStationId(stationId)
	if cfg == nil {
		return nil, fmt.Errorf("carrier %s not found in config", stationId)
	}

	info := &CarrierInfo{
		StationId:     cfg.StationId,
		Name:          cfg.Name,
		InaraId:       cfg.InaraId,
		CurrentSystem: "Unknown",
	}

	// Get runtime state from database
	state := database.FetchCarrierState(stationId)
	if state != nil {
		if state.CurrentSystem != nil {
			info.CurrentSystem = *state.CurrentSystem
		}
		info.LocationUpdated = state.LocationUpdated
		info.LocationChanged = state.LocationChanged
		info.JumpTime = state.JumpTime
		info.Destination = state.Destination
		info.Status = state.Status
		info.PendingJumpDest = state.PendingJumpDest
		info.PendingJumpTime = state.PendingJumpTime
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

// SetCarrierLocation sets the location manually for a carrier
func SetCarrierLocation(stationId string, system string) error {
	if core.Settings.GetCarrierByStationId(stationId) == nil {
		return fmt.Errorf("carrier %s not found", stationId)
	}
	now := time.Now().Unix()
	success, _ := database.UpdateCarrierLocation(stationId, system, "", now)
	if !success {
		return fmt.Errorf("failed to update location")
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
	sb.WriteString("Times shown in your local time\n\n")

	carriers := GetAllCarriersInfo()
	for i, c := range carriers {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(formatSingleCarrier(c))
	}

	sb.WriteString("\n<https://distantworlds3.space/carriers/>")

	return sb.String()
}

// PostCarrierFlightLog posts a carrier update to the flight log channel
func PostCarrierFlightLog(stationId string, changes []string) {
	if core.Settings.DisableFlightLogs() {
		return
	}
	channelId := core.Settings.CarrierFlightLogChannelId()
	if channelId == "" || discordSession == nil {
		return
	}

	info, err := GetCarrierInfo(stationId)
	if err != nil {
		core.LogErrorF("Failed to get carrier info for flight log: %s", err)
		return
	}

	var sb strings.Builder

	// Format the carrier entry
	sb.WriteString(formatSingleCarrier(info))

	// Add what changed
	if len(changes) > 0 {
		sb.WriteString("\n**Changes:** ")
		sb.WriteString(strings.Join(changes, ", "))
	}

	_, err = discordSession.ChannelMessageSend(channelId, sb.String())
	if err != nil {
		core.LogErrorF("Failed to post flight log: %s", err)
	}
}

func formatSingleCarrier(c *CarrierInfo) string {
	var sb strings.Builder

	// Header: NAME - STATION-ID (linked to Inara if available)
	stationIdStr := c.StationId
	if c.InaraId > 0 {
		stationIdStr = fmt.Sprintf("[%s](https://inara.cz/elite/station/%d/)", c.StationId, c.InaraId)
	}
	sb.WriteString(fmt.Sprintf("**%s - %s**\n", strings.ToUpper(c.Name), stationIdStr))

	// Location with timestamps
	locationLine := fmt.Sprintf("\U0001F4CD Last Known Location: %s", c.CurrentSystem) // 📍
	if c.LocationChanged != nil {
		locationLine += fmt.Sprintf(" (changed <t:%d:R>", *c.LocationChanged)
		if c.LocationUpdated != nil && *c.LocationUpdated != *c.LocationChanged {
			locationLine += fmt.Sprintf(", last confirmed <t:%d:R>", *c.LocationUpdated)
		}
		locationLine += ")"
	} else if c.LocationUpdated != nil {
		locationLine += fmt.Sprintf(" (updated <t:%d:R>)", *c.LocationUpdated)
	}
	sb.WriteString(locationLine + "\n")

	now := time.Now().Unix()

	// Pending jump from EDDN (scheduled jump)
	if c.PendingJumpDest != nil && c.PendingJumpTime != nil && *c.PendingJumpTime > now {
		sb.WriteString(fmt.Sprintf("\U0001F4E1 Pending Jump: %s (<t:%d:R>)\n", *c.PendingJumpDest, *c.PendingJumpTime)) // 📡
	}

	// Manual jump time
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
		destLine := fmt.Sprintf("\U0001F4CC Destination: %s", *c.Destination) // 📌
		// Add distance if both systems are known
		if c.CurrentSystem != "Unknown" {
			dist, err := GetDistanceBetweenSystems(c.CurrentSystem, *c.Destination)
			if err == nil && dist >= 0 {
				destLine += fmt.Sprintf(" (%.1f ly)", dist)
			}
		}
		sb.WriteString(destLine + "\n\n")
	}

	// Status (always show if set)
	if c.Status != nil && *c.Status != "" {
		sb.WriteString(fmt.Sprintf("\u2139\uFE0F Status: %s\n\n", *c.Status)) // ℹ️
	}

	// Transit message or no scheduled jump
	if c.JumpTime != nil && *c.JumpTime <= now {
		sb.WriteString("*Please remain seated while the carrier is in transit*\n")
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

// ParseJumpTime parses a time string and returns unix timestamp
// Supports formats:
//   - Unix timestamp (e.g., "1737399000")
//   - "20th January, 18:30 UTC" (assumes current year)
//   - "20th January 2026, 18:30 UTC"
func ParseJumpTime(input string) (int64, error) {
	input = strings.TrimSpace(input)

	// Try unix timestamp first
	if ts, err := strconv.ParseInt(input, 10, 64); err == nil {
		return ts, nil
	}

	// Try parsing human-readable format: "20th January, 18:30 UTC"
	// Pattern: day + ordinal + month + optional year + time (: or .) + UTC + optional suffix (approx, etc.)
	pattern := regexp.MustCompile(`(?i)(\d{1,2})(?:st|nd|rd|th)?\s+(\w+)(?:\s+(\d{4}))?,?\s+(\d{1,2})[:.](\d{2})\s*UTC`)
	matches := pattern.FindStringSubmatch(input)

	if matches == nil {
		return 0, fmt.Errorf("invalid time format. Use: '20th January, 18:30 UTC' or unix timestamp")
	}

	day, _ := strconv.Atoi(matches[1])
	monthStr := matches[2]
	yearStr := matches[3]
	hour, _ := strconv.Atoi(matches[4])
	minute, _ := strconv.Atoi(matches[5])

	// Parse month
	month, err := parseMonth(monthStr)
	if err != nil {
		return 0, err
	}

	// Parse year (default to current year)
	year := time.Now().UTC().Year()
	if yearStr != "" {
		year, _ = strconv.Atoi(yearStr)
	}

	// Build the time in UTC
	t := time.Date(year, month, day, hour, minute, 0, 0, time.UTC)

	return t.Unix(), nil
}

func parseMonth(s string) (time.Month, error) {
	months := map[string]time.Month{
		"january":   time.January,
		"february":  time.February,
		"march":     time.March,
		"april":     time.April,
		"may":       time.May,
		"june":      time.June,
		"july":      time.July,
		"august":    time.August,
		"september": time.September,
		"october":   time.October,
		"november":  time.November,
		"december":  time.December,
		// Short forms
		"jan": time.January,
		"feb": time.February,
		"mar": time.March,
		"apr": time.April,
		"jun": time.June,
		"jul": time.July,
		"aug": time.August,
		"sep": time.September,
		"oct": time.October,
		"nov": time.November,
		"dec": time.December,
	}

	if m, ok := months[strings.ToLower(s)]; ok {
		return m, nil
	}
	return 0, fmt.Errorf("unknown month: %s", s)
}
