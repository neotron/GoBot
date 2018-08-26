package dispatch

import (
	"fmt"
	"strings"

	"GoBot/core"
	"github.com/bwmarrin/discordgo"
)

// MessageCommand is used when registering a handler.
type MessageCommand struct {
	Command string // Command name or prefix
	Help    string // Help string
}

// Container for a message, session and parsed arguments.
type Message struct {
	*discordgo.Message
	*discordgo.Session
	Command string
	Args    []string
}

type Test interface {
	cat() string
}

// Utility method to send quick reply back to the channel
func (m Message) ReplyToChannel(format string, v ...interface{}) {
	m.ChannelMessageSend(m.ChannelID, fmt.Sprintf(format, v...))
}

// Utility method to send a reply to the author of the message
func (m Message) ReplyToSender(format string, v ...interface{}) {
	go func() {
		ch, err := m.UserChannelCreate(m.Author.ID)
		if err != nil {
			core.LogError("Failed to open private channel: ", err)
		}
		m.ChannelMessageSend(ch.ID, fmt.Sprintf(format, v...))
	}()
}

// Interface used for message handlers
type MessageHandler interface {
	// Process requests for Command with this prefix.
	handlePrefix(string, *Message) bool
	// Process Command requests for the specific Command.
	handleCommand(*Message) bool
	// Wildcard handling for any Command.
	handleAnything(*Message) bool
	// Optional group for this command
	CommandGroup() string
}

// Each message handler can process one or more commands / message responses
type NoOpMessageHandler struct{}

func (*NoOpMessageHandler) CommandGroup() string {
	return ""
}

func (*NoOpMessageHandler) handlePrefix(string, *Message) bool {
	return false
}

func (*NoOpMessageHandler) handleCommand(*Message) bool {
	return false
}

func (*NoOpMessageHandler) handleAnything(*Message) bool {
	return false
}

func toName(handler MessageHandler) string {
	return strings.TrimPrefix(fmt.Sprintf("%T", handler), "*")
}
