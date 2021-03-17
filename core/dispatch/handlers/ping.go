package handlers

import (
	"fmt"
	"time"

	"GoBot/core/dispatch"
)

var bootTime = time.Now()

type ping struct {
	dispatch.NoOpMessageHandler
}


func init() {
	dispatch.Register(&ping{},
		[]dispatch.MessageCommand{
			{"ping", "Simple command to check that bot is alive"},
			{"pong", "Simple command to check that bot is alive"},
			{"pingme", "Send a ping on a private message."},
			{"uptime", "Get bot uptime information."},
		},
		[]dispatch.MessageCommand{{"test", "Simple test prefix command"}},
		true)
}

func (*ping) CommandGroup() string {
	return "Test Commands"
}

func handleUptime(m *dispatch.Message) {
	uptime := time.Since(bootTime)
	sec := uint64(uptime.Seconds())
	days :=  sec / 86400
	sec %= 86400
	hours := sec / 3600
	sec %= 3600
	min := sec / 60
	sec %= 60

	reply := ""
	if days > 0 {
		reply += fmt.Sprint(days, "d ")
	}
	if hours > 0 {
		reply += fmt.Sprint(hours, "h ")
	}
	if min > 0 {
		reply += fmt.Sprint(min, "m ")
	}
	if sec > 0 {
		reply += fmt.Sprint(sec, "s")
	}
	m.ReplyToChannel("Uptime: %s.\nMy local time is: %s", reply, time.Now().Local())

}

func (*ping) HandleCommand(m *dispatch.Message) bool {
	switch m.Command {
	case "ping":
		m.ReplyToChannel("Pong!")
	case "pong":
		m.ReplyToChannel("Ping!")
	case "pingme":
		m.ReplyToSender("Ping!")
	case "uptime":
		handleUptime(m)
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
