package core

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/jcelliott/lumber"
)

// CarrierConfig defines a fleet carrier from config
type CarrierConfig struct {
	StationId string `json:"stationId"` // Carrier callsign e.g. "W7H-6DZ"
	Name      string `json:"name"`      // Display name e.g. "DSEV Odysseus"
	InaraId   int    `json:"inaraId"`   // Inara station ID for linking (optional)
}

type jsonData struct {
	LogLevel               string // "TRACE", "DEBUG", "INFO", "WARN", "ERROR" (default: INFO)
	AuthToken              string
	CommandPrefix          string
	Database               string
	ResourceDirectory      string
	OwnerIds               []string
	CustomCommandCooldown  int             // Cooldown in seconds between same custom command uses (0 = no cooldown)
	BotChannels            []string        // Channel IDs for bot-spam (cooldown exempt, carriers reply in channel)
	CarrierOwnerIds        []string        // Discord user IDs who can manage carriers
	Carriers               []CarrierConfig // Fleet carrier definitions
	SlashCommandGuildId      string          // Guild ID for slash commands (empty = global, can take 1hr to propagate)
	CarrierUpdateChannelId   string          // Channel ID to watch for carrier status updates
	CarrierFlightLogChannelId    string  // Channel ID to post carrier change logs
	FollowerDistanceThreshold float64 // Distance in ly to consider a carrier "following" (default 100)
	DisableFlightLogs     bool     // When true, don't post carrier updates to Discord
	SlashCommandAllowlist []string // When non-empty, only register these slash commands
	CarrierValidation     []string // Validation modes: "range" (distance check), "time" (cooldown check). Empty = no validation
}

type SettingsStorage struct {
	data jsonData
}

var Settings = SettingsStorage{jsonData{}}

// Load the settings from a json file and stuff it into a new SettingsStorage object.
func LoadSettings(settingsfile string) {
	file, err := os.Open(settingsfile)
	if err != nil {
		LogFatal("Failed to open config file: ", err)

	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&Settings.data)
	if err != nil {
		LogFatal("Failed to parse configuration: ", err)
	}

	// Set log level from config (default to INFO)
	switch strings.ToUpper(Settings.data.LogLevel) {
	case "TRACE":
		SetLogLevel(lumber.TRACE)
	case "DEBUG":
		SetLogLevel(lumber.DEBUG)
	case "WARN":
		SetLogLevel(lumber.WARN)
	case "ERROR":
		SetLogLevel(lumber.ERROR)
	default:
		SetLogLevel(lumber.INFO)
	}

	if Settings.data.DisableFlightLogs {
		LogInfo("Flight logs disabled (disableFlightLogs=true)")
	}
	if len(Settings.data.SlashCommandAllowlist) > 0 {
		LogInfoF("Slash command allowlist configured: %v", Settings.data.SlashCommandAllowlist)
	}
	if len(Settings.data.CarrierValidation) > 0 {
		LogInfoF("Carrier validation enabled: %v", Settings.data.CarrierValidation)
	}

	LogDebug("Loaded config successfully from ", settingsfile)
}

// Get location of resources
func (s *SettingsStorage) ResourceDirectory() string {
	return s.data.ResourceDirectory
}
// Get the bot auth tooken
func (s *SettingsStorage) AuthToken() string {
	return s.data.AuthToken
}

// Get the prefix used for bot commands
func (s *SettingsStorage) CommandPrefix() string {
	return s.data.CommandPrefix
}

// Directory database is stored in
func (s *SettingsStorage) Database() string {
	return s.data.Database
}

// CustomCommandCooldown returns the cooldown in seconds between uses of the same custom command
func (s *SettingsStorage) CustomCommandCooldown() int {
	return s.data.CustomCommandCooldown
}

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

// SetTestCarriers sets carrier config for testing purposes
func (s *SettingsStorage) SetTestCarriers(carriers []CarrierConfig) {
	s.data.Carriers = carriers
}

// SlashCommandGuildId returns the guild ID for slash command registration (empty = global)
func (s *SettingsStorage) SlashCommandGuildId() string {
	return s.data.SlashCommandGuildId
}

// CarrierUpdateChannelId returns the channel ID to watch for carrier status updates
func (s *SettingsStorage) CarrierUpdateChannelId() string {
	return s.data.CarrierUpdateChannelId
}

// CarrierFlightLogChannelId returns the channel ID for carrier change logs
func (s *SettingsStorage) CarrierFlightLogChannelId() string {
	return s.data.CarrierFlightLogChannelId
}

// FollowerDistanceThreshold returns the distance threshold for follower detection (default 100 ly)
func (s *SettingsStorage) FollowerDistanceThreshold() float64 {
	if s.data.FollowerDistanceThreshold <= 0 {
		return 100.0 // default
	}
	return s.data.FollowerDistanceThreshold
}

// DisableFlightLogs returns whether flight log posting is disabled
func (s *SettingsStorage) DisableFlightLogs() bool {
	return s.data.DisableFlightLogs
}

// SlashCommandAllowlist returns the list of allowed slash commands (empty = all allowed)
func (s *SettingsStorage) SlashCommandAllowlist() []string {
	return s.data.SlashCommandAllowlist
}

// CarrierValidationEnabled checks if a specific validation mode is enabled
// Valid modes: "range" (distance check), "time" (cooldown check)
func (s *SettingsStorage) CarrierValidationEnabled(mode string) bool {
	for _, v := range s.data.CarrierValidation {
		if strings.EqualFold(v, mode) {
			return true
		}
	}
	return false
}
