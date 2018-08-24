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

func (handler *PingHandler) commands() []MessageCommand {
	return []MessageCommand{
		{"ping", "Simple command to check that bot is alive"},
		{"pong", "Simple command to check that bot is alive"},
	}
}

func (handler *PingHandler) handleCommand(command string, args []string, session *discordgo.Session, message *discordgo.Message) bool {
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
