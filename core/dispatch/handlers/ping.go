package handlers

import (
	"GoBot/core/dispatch"
)

type ping struct {
	dispatch.NoOpMessageHandler
}

func init() {
	dispatch.Register(&ping{},
		[]dispatch.MessageCommand{
			{"ping", "Simple command to check that bot is alive"},
			{"pong", "Simple command to check that bot is alive"},
			{"pingme", "Send a ping on a private message."},
		},
		[]dispatch.MessageCommand{{"test", "Simple test prefix command"}},
		true)
}

func (*ping) CommandGroup() string {
	return "Test Commands"
}

func (*ping) HandleCommand(m *dispatch.Message) bool {
	switch m.Command {
	case "ping":
		m.ReplyToChannel("Pong!")
	case "pong":
		m.ReplyToChannel("Ping!")
	case "pingme":
		m.ReplyToSender("Ping!")
	default:
		return false
	}
	return true
}

func (*ping) HandlePrefix(prefix, suffix string, m *dispatch.Message) bool {
	switch suffix {
	case "ping":
		m.ReplyToChannel("Pong!")
	case "pong":
		m.ReplyToChannel("Ping!")
	default:
		return false
	}
	return true
}

func (*ping) HandleAnything(m *dispatch.Message) bool {
	switch m.Command {
	case "anyping":
		m.ReplyToChannel("Pong!")
	case "anypong":
		m.ReplyToChannel("Ping!")
	default:
		return false
	}
	return true
}
