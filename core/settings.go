package core

import (
	"encoding/json"
	"log"
	"os"
)

type jsonData struct {
	Development       bool
	AuthToken         string
	CommandPrefix     string
	DatabaseDirectory string
	ResourceDirectory string
	OwnerIds          []string
}

type SettingsStorage struct {
	data jsonData
}

var Settings = SettingsStorage{jsonData{}}

// Load the settings from a json file and stuff it into a new SettingsStorage object.
func LoadSettings(settingsfile string) {
	file, err := os.Open(settingsfile)
	if err != nil {
		log.Fatalln("Failed to open config file", err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&Settings.data)
	if err != nil {
		log.Fatalln("Failed to parse configuration: ", err)
	}
	log.Println("found config: ", Settings)
}

// Get the bot auth tooken
func (s *SettingsStorage) AuthToken() string {
	return s.data.AuthToken
}

// Get the prefix used for bot commands
func (s *SettingsStorage) CommandPrefix() string {
	return s.data.CommandPrefix
}

// Get wXShether or not we're running in Development mode.
func (s *SettingsStorage) IsDevelopment() bool {
	return s.data.Development
}
