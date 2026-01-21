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
	CarrierFlightLogChannelId string         // Channel ID to post carrier change logs
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
