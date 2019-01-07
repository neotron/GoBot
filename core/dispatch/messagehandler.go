package dispatch

import (
	"fmt"
	"strings"

	"GoBot/core"
	"github.com/bwmarrin/discordgo"
)

type Flags int

const (
	None    Flags = 0
	Verbose Flags = 1 << iota
	Help
	Here
)

func (flags Flags) IsSet(check Flags) bool {
	return (flags & check) == check
}

// MessageCommand is used when registering a handler.
type MessageCommand struct {
	Command string // Command name or prefix
	Help    string // Help string
}

// Container for a message, session and parsed arguments.
type Message struct {
	*discordgo.Message
	*discordgo.Session
	Command       string
	Args, RawArgs []string
	Flags         Flags
	IsPM          bool
}

type Test interface {
	cat() string
}

// Utility method to send quick reply back to the channel
func (m Message) ReplyToChannel(format string, v ...interface{}) {
	m.ChannelMessageSend(m.ChannelID, fmt.Sprintf(format, v...))
}

// Utility method to send a reply to the author of the message
func (m Message) ReplyToSender(format string, v ...interface{}) chan struct{} {
	sendDone := make(chan struct{})
	go func() {
		ch, err := m.UserChannelCreate(m.Author.ID)
		if err != nil {
			core.LogError("Failed to open private channel: ", err)
		}
		m.ChannelMessageSend(ch.ID, fmt.Sprintf(format, v...))
		sendDone <- struct{}{}
	}()
	return sendDone
}

// Interface used for message handlers
type MessageHandler interface {
	// Process requests for Command with this prefix.
	HandlePrefix(string, string, *Message) bool
	// Process Command requests for the specific Command.
	HandleCommand(*Message) bool
	// Wildcard handling for any Command.
	HandleAnything(*Message) bool
	// Optional group for this command
	CommandGroup() string
	// Called when settings file are loaded
	SettingsLoaded()
}

// Each message handler can process one or more commands / message responses
type NoOpMessageHandler struct{}

func (*NoOpMessageHandler) CommandGroup() string {
	return ""
}

func (*NoOpMessageHandler) SettingsLoaded() {
}

func (*NoOpMessageHandler) HandlePrefix(string, string, *Message) bool {
	return false
}

func (*NoOpMessageHandler) HandleCommand(*Message) bool {
	return false
}

func (*NoOpMessageHandler) HandleAnything(*Message) bool {
	return false
}

func toName(handler MessageHandler) string {
	return strings.TrimPrefix(fmt.Sprintf("%T", handler), "*")
}
