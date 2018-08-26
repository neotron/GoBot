package handlers

import (
	"github.com/bwmarrin/discordgo"
	"github.com/neotron/GoBot/dispatch"
)

type ping struct {
	dispatch.MessageHandler
}

func init() {
	dispatch.Register(&ping{dispatch.MessageHandler{}},
		[]dispatch.MessageCommand{
			{"ping", "Simple command to check that bot is alive"},
			{"pong", "Simple command to check that bot is alive"},
		},
		[]dispatch.MessageCommand{{"test", "Simple test prefix command"}},
		false)
}

func (*ping) handleCommand(command string, args []string, session *discordgo.Session, message *discordgo.Message) bool {
	processed := false
	switch command {
	case "ping":
		session.ChannelMessageSend(message.ChannelID, "Pong!")
		processed = true
	case "pong":
		session.ChannelMessageSend(message.ChannelID, "Ping!")
		processed = true
	}
	return processed
}

func (*ping) handlePrefix(prefix string, command string, args []string, session *discordgo.Session, message *discordgo.Message) bool {
	processed := false
	if prefix == "test" {
		switch command {
		case "testping":
			session.ChannelMessageSend(message.ChannelID, "Pong!")
			processed = true
		case "testpong":
			session.ChannelMessageSend(message.ChannelID, "Ping!")
			processed = true
		}
	}
	return processed
}
