package dispatch

import "github.com/bwmarrin/discordgo"

// MessageCommand is used when registering a handler.
type MessageCommand struct {
	Command string // Command name or prefix
	Help    string // Help string
}

type IMessageHandler interface {
	// Process requests for command with this prefix.
	handlePrefix(prefix string, command string, session []string, sess *discordgo.Session, message *discordgo.Message) bool
	// Process command requests for the specific command.
	handleCommand(command string, args []string, session *discordgo.Session, message *discordgo.Message) bool
	// Wildcard handling for any command.
	handleAnything(command string, args []string, session *discordgo.Session, message *discordgo.Message) bool
}

// Each message handler can process one or more commands / message responses
type MessageHandler struct{}

func (*MessageHandler) handlePrefix(prefix string, command string, session []string, sess *discordgo.Session, message *discordgo.Message) bool {
	return false
}

func (*MessageHandler) handleCommand(command string, args []string, session *discordgo.Session, message *discordgo.Message) bool {
	return false
}

func (*MessageHandler) handleAnything(command string, args []string, session *discordgo.Session, message *discordgo.Message) bool {
	return false
}

func (handler *MessageHandler) prefixes() []MessageCommand { return nil }
func (handler *MessageHandler) commands() []MessageCommand { return nil }
func (handler *MessageHandler) commandGroup() string       { return "" }
func (handler *MessageHandler) isWildCard() bool           { return false }
