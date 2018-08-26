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
		},
		[]dispatch.MessageCommand{{"test", "Simple test prefix command"}},
		true)
}

func (*ping) handleCommand(m *dispatch.Message) bool {
	processed := false
	switch m.Command {
	case "ping":
		m.ReplyToChannel("Pong!")
		processed = true
	case "pong":
		m.ReplyToChannel("Ping!")
		processed = true
	}
	return processed
}

func (*ping) handlePrefix(prefix string, m *dispatch.Message) bool {
	processed := false
	switch m.Command {
	case "testping":
		m.ReplyToChannel("Pong!")
		processed = true
	case "testpong":
		m.ReplyToChannel("Ping!")
		processed = true
	}
	return processed
}

func (*ping) handleAnything(m *dispatch.Message) bool {
	processed := false
	switch m.Command {
	case "anyping":
		m.ReplyToChannel("Pong!")
		processed = true
	case "anypong":
		m.ReplyToChannel("Ping!")
		processed = true
	}
	return processed
}
