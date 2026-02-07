package services

import (
	"regexp"
	"strings"
	"unicode"

	"GoBot/core"

	"github.com/bwmarrin/discordgo"
)

// CarrierUpdate represents parsed carrier data from a channel message
type CarrierUpdate struct {
	StationId   string
	Departure   *int64
	Destination *string
}

var (
	// Matches station ID pattern: XXX-XXX (3 alphanumeric, dash, 3 alphanumeric)
	stationIdPattern = regexp.MustCompile(`[A-Z0-9]{3}-[A-Z0-9]{3}`)

	// Line patterns for extracting fields
	departurePattern   = regexp.MustCompile(`(?i)^Departure:\s*(.+)$`)
	destinationPattern = regexp.MustCompile(`(?i)^Destination(?:\s*System)?:\s*(.+)$`)

	// Matches the sector + mass code part of a procedurally generated system name
	// e.g. "MT-Q e5-8" in "Thuecheae MT-Q e5-8"
	procGenSectorPattern = regexp.MustCompile(`(?i)([A-Z]{2}-[A-Z])\s+([A-Z]\d+-\d+)`)
)

// findCarrierByName finds a carrier config by name (case-insensitive)
func findCarrierByName(name string) *core.CarrierConfig {
	nameLower := strings.ToLower(strings.TrimSpace(name))
	for _, c := range core.Settings.Carriers() {
		if strings.ToLower(c.Name) == nameLower {
			return &c
		}
	}
	return nil
}

// ProcessCarrierUpdateMessage processes a message from the carrier update channel
func ProcessCarrierUpdateMessage(authorId, channelId, content string) {
	// Check if this is the carrier update channel
	configuredChannel := core.Settings.CarrierUpdateChannelId()
	if channelId != configuredChannel {
		return
	}

	// Check if author is a carrier owner
	if !core.Settings.IsCarrierOwner(authorId) {
		core.LogDebugF("Carrier update from non-owner %s, ignoring", authorId)
		return
	}

	// Parse carrier updates from message
	updates := ParseCarrierUpdates(content)
	if len(updates) == 0 {
		return
	}

	// Process each carrier update
	for _, update := range updates {
		processCarrierUpdate(&update)
	}
}

// ProcessCarrierUpdateChannelOnStartup fetches and processes recent messages from the carrier update channel
func ProcessCarrierUpdateChannelOnStartup(s *discordgo.Session) {
	channelId := core.Settings.CarrierUpdateChannelId()
	if channelId == "" {
		return
	}

	// Fetch recent messages from the channel (newest first from Discord API)
	messages, err := s.ChannelMessages(channelId, 20, "", "", "")
	if err != nil {
		core.LogErrorF("Failed to fetch carrier update channel messages: %s", err)
		return
	}

	core.LogInfoF("Processing %d messages from carrier update channel on startup", len(messages))

	// Process messages (they come in reverse chronological order, but order doesn't matter for us)
	for _, msg := range messages {
		if msg.Author == nil {
			continue
		}
		ProcessCarrierUpdateMessage(msg.Author.ID, channelId, msg.Content)
	}
}

// ParseCarrierUpdates parses carrier blocks from message content
func ParseCarrierUpdates(content string) []CarrierUpdate {
	var updates []CarrierUpdate

	// Remove markdown code block markers and Unicode formatting characters
	content = strings.ReplaceAll(content, "```", "")
	content = stripUnicodeFormatting(content)

	// Split by "Carrier:" (case-insensitive) to find carrier blocks
	// Use regex to split while preserving case
	carrierPattern := regexp.MustCompile(`(?i)Carrier:`)
	indices := carrierPattern.FindAllStringIndex(content, -1)

	if len(indices) == 0 {
		return updates
	}

	for i, idx := range indices {
		// Extract block from this "Carrier:" to the next one (or end)
		start := idx[0]
		end := len(content)
		if i+1 < len(indices) {
			end = indices[i+1][0]
		}

		block := content[start:end]
		update := parseCarrierBlock(block)
		if update != nil {
			updates = append(updates, *update)
		}
	}

	return updates
}

// parseCarrierBlock parses a single carrier block
func parseCarrierBlock(block string) *CarrierUpdate {
	lines := strings.Split(block, "\n")
	if len(lines) == 0 {
		return nil
	}

	// Extract station ID from first line (Carrier: name STATION-ID)
	// or match by carrier name if no station ID present
	firstLine := lines[0]
	firstLineUpper := strings.ToUpper(firstLine)
	stationIdMatch := stationIdPattern.FindString(firstLineUpper)

	var stationId string
	if stationIdMatch != "" {
		// Found station ID pattern
		if core.Settings.GetCarrierByStationId(stationIdMatch) == nil {
			return nil
		}
		stationId = stationIdMatch
	} else {
		// No station ID, try to match by carrier name
		// "Carrier: DSEV Odysseus" -> extract "DSEV Odysseus"
		carrierName := strings.TrimSpace(strings.TrimPrefix(firstLine, "Carrier:"))
		// Also try case-insensitive prefix removal
		if strings.HasPrefix(strings.ToLower(firstLine), "carrier:") {
			carrierName = strings.TrimSpace(firstLine[8:]) // len("Carrier:") = 8
		}
		carrierName = strings.Trim(carrierName, " \t\r\n")

		cfg := findCarrierByName(carrierName)
		if cfg == nil {
			return nil
		}
		stationId = cfg.StationId
	}

	update := &CarrierUpdate{
		StationId: stationId,
	}

	// Parse remaining lines for fields
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Try to match Departure
		if match := departurePattern.FindStringSubmatch(line); match != nil {
			timeStr := strings.TrimSpace(match[1])
			// Check if this indicates clearing the departure
			if isClearValue(timeStr) || isPlaceholder(timeStr) {
				zero := int64(0)
				update.Departure = &zero // 0 = clear
			} else {
				// Try to parse as a time
				if ts, err := ParseJumpTime(timeStr); err == nil {
					update.Departure = &ts
				} else {
					core.LogDebugF("Failed to parse departure time '%s', clearing: %s", timeStr, err)
					zero := int64(0)
					update.Departure = &zero // 0 = clear unparseable departure
				}
			}
			continue
		}

		// Try to match Destination System
		if match := destinationPattern.FindStringSubmatch(line); match != nil {
			destStr := strings.TrimSpace(match[1])
			// Clear destination if placeholder/invalid, otherwise set it
			if isClearValue(destStr) || isPlaceholder(destStr) {
				empty := ""
				update.Destination = &empty // empty = clear
			} else {
				update.Destination = &destStr
			}
			continue
		}
	}

	return update
}

// isPlaceholder checks if a value is a placeholder that should be skipped (no change)
func isPlaceholder(s string) bool {
	lower := strings.ToLower(s)

	// Exact matches
	switch lower {
	case "", "[processing]", "[error]", "tbd", "tba", "n/a", "pending", "---", "???", "underway":
		return true
	}

	// Partial matches for in-progress states
	if strings.Contains(lower, "in transit") ||
		strings.Contains(lower, "in progress") ||
		strings.Contains(lower, "processing") ||
		strings.Contains(lower, "pending") ||
		strings.Contains(lower, "underway") {
		return true
	}

	return false
}

// isClearValue checks if a value indicates the field should be cleared
func isClearValue(s string) bool {
	lower := strings.ToLower(s)

	// "None" or variations like "None / Not Scheduled"
	if lower == "none" || strings.HasPrefix(lower, "none ") || strings.HasPrefix(lower, "none/") {
		return true
	}
	if strings.Contains(lower, "not scheduled") {
		return true
	}

	return false
}

// ExtractProcGenSystemName extracts a procedurally generated Elite Dangerous system name
// from text that may contain extra words (e.g. "Heading towards Thuecheae MT-Q e5-8").
// Matches the sector code pattern (e.g. "MT-Q") followed by mass code (e.g. "e5-8"),
// then walks backwards to grab region name words that start with an uppercase letter.
// Returns empty string if no procedural name found.
func ExtractProcGenSystemName(text string) string {
	loc := procGenSectorPattern.FindStringIndex(text)
	if loc == nil {
		return ""
	}

	sectorMass := text[loc[0]:loc[1]]

	// Walk backwards from sector code to find region name words
	prefix := strings.TrimRight(text[:loc[0]], " \t")
	words := strings.Fields(prefix)

	if len(words) == 0 {
		return sectorMass
	}

	// Grab trailing words that are purely alphabetic and start with uppercase
	// (region names like "Thuecheae", "Blu Thua" — not English words like "towards")
	var regionWords []string
	for i := len(words) - 1; i >= 0 && i >= len(words)-4; i-- {
		word := words[i]
		if isAlphaWord(word) && len(word) > 0 && unicode.IsUpper(rune(word[0])) {
			regionWords = append([]string{word}, regionWords...)
		} else {
			break
		}
	}

	if len(regionWords) == 0 {
		return sectorMass
	}

	return strings.Join(regionWords, " ") + " " + sectorMass
}

// stripUnicodeFormatting removes invisible Unicode formatting characters that
// Discord may insert (directional isolates, embedding marks, zero-width spaces, etc.)
func stripUnicodeFormatting(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= '\u200B' && r <= '\u200F': // zero-width spaces, LTR/RTL marks
			return -1
		case r >= '\u2028' && r <= '\u202F': // line/paragraph separators, directional embedding
			return -1
		case r >= '\u2066' && r <= '\u2069': // directional isolates
			return -1
		case r == '\uFEFF': // BOM / zero-width no-break space
			return -1
		default:
			return r
		}
	}, s)
}

// isAlphaWord checks if a string contains only letters
func isAlphaWord(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		if !unicode.IsLetter(c) {
			return false
		}
	}
	return true
}

// processCarrierUpdate applies a carrier update if values have changed
func processCarrierUpdate(update *CarrierUpdate) {
	info, err := GetCarrierInfo(update.StationId)
	if err != nil {
		core.LogErrorF("Failed to get carrier info for %s: %s", update.StationId, err)
		return
	}

	var changes []string

	// Update departure if changed
	if update.Departure != nil {
		currentJump := int64(0)
		if info.JumpTime != nil {
			currentJump = *info.JumpTime
		}

		if *update.Departure == 0 {
			// Clear jump time (0 is sentinel for "None")
			if currentJump != 0 {
				if err := ClearCarrierField(update.StationId, "jump"); err != nil {
					core.LogErrorF("Failed to clear jump time for %s: %s", update.StationId, err)
				} else {
					core.LogInfoF("Channel update: Carrier %s jump time cleared", update.StationId)
					changes = append(changes, "jump time cleared")
				}
			}
		} else if *update.Departure != currentJump {
			if err := SetCarrierJumpTime(update.StationId, *update.Departure); err != nil {
				core.LogErrorF("Failed to set jump time for %s: %s", update.StationId, err)
			} else {
				core.LogInfoF("Channel update: Carrier %s jump time set to %d", update.StationId, *update.Departure)
				changes = append(changes, "jump time updated")
			}
		}
	}

	// Update destination if changed
	if update.Destination != nil {
		currentDest := ""
		if info.Destination != nil {
			currentDest = *info.Destination
		}

		if *update.Destination == "" {
			// Clear destination (empty string is sentinel for "clear")
			if currentDest != "" {
				if err := ClearCarrierField(update.StationId, "dest"); err != nil {
					core.LogErrorF("Failed to clear destination for %s: %s", update.StationId, err)
				} else {
					core.LogInfoF("Channel update: Carrier %s destination cleared", update.StationId)
					changes = append(changes, "destination cleared")
				}
			}
		} else if *update.Destination != currentDest {
			// Set destination (no EDSM validation required — non-system names like
			// "Waypoint 3" are allowed; distance just won't be shown)
			if err := SetCarrierDestination(update.StationId, *update.Destination); err != nil {
				core.LogErrorF("Failed to set destination for %s: %s", update.StationId, err)
			} else {
				core.LogInfoF("Channel update: Carrier %s destination set to %s", update.StationId, *update.Destination)
				changes = append(changes, "destination updated")
			}
		}
	}

	// Post flight log if any changes were made
	if len(changes) > 0 {
		PostCarrierFlightLog(update.StationId, changes)
	}
}
