package services

import (
	"fmt"
	"strings"
	"time"

	"GoBot/core/database"
)

const (
	defaultFollowerDays  = 7
	defaultMinSightings  = 1 // "more than 1" means 2+ sightings
	inaraSearchURL       = "https://inara.cz/elite/search/?search=%s"
)

// formatCarrierLink formats a carrier ID as an Inara search link (no embed)
func formatCarrierLink(stationId string) string {
	return fmt.Sprintf("[%s](<"+inaraSearchURL+">)", stationId, stationId)
}

// GetRecentFollowers returns formatted list of followers for display
func GetRecentFollowers(sortBy string) []database.CarrierFollower {
	return database.FetchRecentFollowers(defaultFollowerDays, defaultMinSightings, sortBy)
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
		avgDist := 0.0
		if f.TimesSeen > 0 {
			avgDist = f.TotalDistance / float64(f.TimesSeen)
		}
		sb.WriteString(fmt.Sprintf("**%s** - %d sightings\n", formatCarrierLink(f.FollowerStationId), f.TimesSeen))
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

	avgDist := 0.0
	if f.TimesSeen > 0 {
		avgDist = f.TotalDistance / float64(f.TimesSeen)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**CARRIER INFO: %s**\n\n", formatCarrierLink(f.FollowerStationId)))
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
