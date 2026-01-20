package core

import (
	"encoding/json"
	"os"

	"github.com/jcelliott/lumber"
)

type jsonData struct {
	Development                  bool
	AuthToken                    string
	CommandPrefix                string
	Database                     string
	ResourceDirectory            string
	OwnerIds                     []string
	CustomCommandCooldown        int      // Cooldown in seconds between same custom command uses (0 = no cooldown)
	CooldownWhitelistChannels    []string // Channel IDs exempt from cooldown (e.g., bot-spam channels)
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
	if !Settings.IsDevelopment() {
		SetLogLevel(lumber.INFO)
	} else {
		LogDebug("Loaded config successfully from ", settingsfile)
	}

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

// Get whether or not we're running in Development mode.
func (s *SettingsStorage) IsDevelopment() bool {
	return s.data.Development
}

// Directory database is stored in
func (s *SettingsStorage) Database() string {
	return s.data.Database
}

// CustomCommandCooldown returns the cooldown in seconds between uses of the same custom command
func (s *SettingsStorage) CustomCommandCooldown() int {
	return s.data.CustomCommandCooldown
}

// CooldownWhitelistChannels returns the list of channel IDs exempt from cooldown
func (s *SettingsStorage) CooldownWhitelistChannels() []string {
	return s.data.CooldownWhitelistChannels
}

// IsChannelCooldownWhitelisted checks if a channel ID is exempt from cooldown
func (s *SettingsStorage) IsChannelCooldownWhitelisted(channelID string) bool {
	for _, id := range s.data.CooldownWhitelistChannels {
		if id == channelID {
			return true
		}
	}
	return false
}
