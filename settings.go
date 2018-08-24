package main

import (
	"encoding/json"
	"fmt"
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

type Settings struct {
	data jsonData
}

// Load the settings from a json file and stuff it into a new Settings object.
func LoadSettings(settingsfile string) *Settings {
	file, err := os.Open(settingsfile)
	defer file.Close()
	if err != nil {
		fmt.Println("Failed to open file", err)
		panic(err)
	}
	decoder := json.NewDecoder(file)
	settings := new(Settings)
	err = decoder.Decode(&settings.data)
	if err != nil {
		fmt.Println("Failed to parse configuration: ", err)
		panic(err)
	}
	fmt.Println("found config: ", settings)
	return settings
}

// Get the bot auth tooken
func (s *Settings) AuthToken() string {
	return s.data.AuthToken
}

// Get the prefix used for bot commands
func (s *Settings) CommandPrefix() string {
	return s.data.CommandPrefix
}

// Get wXShether or not we're running in development mode.
func (s *Settings) IsDevelopment() bool {
	return s.data.Development
}
