package main

import "github.com/bwmarrin/discordgo"

type PingHandler struct {
	MessageHandler
}

func RegisterPingHandler(dispatcher *MessageDispatcher) {
	pingHandler := new(PingHandler)
	pingHandler.MessageHandler = MessageHandler{}
	dispatcher.Register(pingHandler)
}

func (*PingHandler) commands() []MessageCommand {
	return []MessageCommand{
		{"ping", "Simple command to check that bot is alive"},
		{"pong", "Simple command to check that bot is alive"},
	}
}

func (*PingHandler) prefixes() []MessageCommand {
	return []MessageCommand{
		{"test", "Simple test prefix command"},
	}
}

func (*PingHandler) handleCommand(command string, args []string, session *discordgo.Session, message *discordgo.Message) bool {
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

func (*PingHandler) handlePrefix(prefix string, command string, args []string, session *discordgo.Session, message *discordgo.Message) bool {
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
