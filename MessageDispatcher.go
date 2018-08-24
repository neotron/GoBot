package main

import (
	"github.com/thoas/go-funk"
	"github.com/bwmarrin/discordgo"
	"fmt"
	"strings"
)

type MessageDispatcher struct {
	settings *Settings
}

func CreateDispatcher(settings *Settings) * MessageDispatcher {
	dispatcher := new(MessageDispatcher)
	dispatcher.settings = settings
	return dispatcher
}

func (dispatcher * MessageDispatcher) Handle(s *discordgo.Session, m *discordgo.Message) {
	fmt.Println("Got message: ", m.Content)

	// Ignore all messages created by the bot itself
	// This isn't required in this specific example but it's a good practice.
	if m.Author.ID == s.State.User.ID {
		return
	}

	// Ensure that the string has the prefix we're programmed to listen to
	trimmed := strings.TrimPrefix(m.Content, dispatcher.settings.CommandPrefix())
	if trimmed == m.Content {
		return
	}

	// Split the command into parameters, and clean them up.
	params := funk.FilterString(strings.Split(trimmed, " "), func(str string) bool {
		return strings.Trim(str, "\t\r") != ""
	})

	// Just a bunch of whitespaces
	if len(params) == 0 {
		return
	}

	fmt.Println("Parsed parameters:", params)
	switch(params[0]) {
	case "ping":
		s.ChannelMessageSend(m.ChannelID, "Pong!")
	case "pong":
		s.ChannelMessageSend(m.ChannelID, "Ping!")
	}

}
