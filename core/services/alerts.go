package services

import (
	"fmt"

	"GoBot/core"
	"GoBot/core/database"
)

// CheckProximityAlerts checks all active proximity alerts against a carrier's new location.
// If a carrier has jumped within range of an alert's target system, a DM is sent and the alert is deleted.
func CheckProximityAlerts(stationId, system string) {
	if discordSession == nil {
		return
	}

	alerts := database.FetchAllProximityAlerts()
	if len(alerts) == 0 {
		return
	}

	// Get coords for the carrier's new system
	carrierCoords, err := GetSystemCoords(system)
	if err != nil || carrierCoords == nil {
		return
	}

	carrierName := getCarrierDisplayName(stationId)

	for _, alert := range alerts {
		alertCoords, err := GetSystemCoords(alert.SystemName)
		if err != nil || alertCoords == nil {
			continue
		}

		distance := CalculateDistance(carrierCoords, alertCoords)
		if distance < 0 || distance > alert.DistanceLY {
			continue
		}

		// Alert triggered — send DM and delete
		msg := fmt.Sprintf("**Carrier Alert:** %s has jumped to **%s**, which is **%.1f ly** from your alert system **%s**.",
			carrierName, system, distance, alert.SystemName)

		ch, err := discordSession.UserChannelCreate(alert.UserID)
		if err != nil {
			core.LogErrorF("Failed to open DM channel for proximity alert user %s: %s", alert.UserID, err)
			continue
		}

		_, err = discordSession.ChannelMessageSend(ch.ID, msg)
		if err != nil {
			core.LogErrorF("Failed to send proximity alert DM to user %s: %s", alert.UserID, err)
			continue
		}

		core.LogInfoF("Proximity alert fired: user %s notified about %s near %s (%.1f ly)", alert.UserID, carrierName, alert.SystemName, distance)
		database.DeleteProximityAlertByID(alert.ID)
	}
}
