package main

import "github.com/bwmarrin/discordgo"

type MessageCommand struct {
	Command string
	Help    string
}

type IMessageHandler interface {
	// For registration

	// Available command prefixes this handler can deal with.
	// A prefix can be a partial world for example:
	// randomcat and randomdog can be handled with a prefix random
	prefixes() []MessageCommand
	// Exact commands
	commands() []MessageCommand
	// Which command group this handler belongs to
	commandGroup() string
	// Whether or not this handler does parsing of any command
	isWildCard() bool

	// Acting on commands
	handlePrefix(prefix string, command string, session []string, sess *discordgo.Session, message *discordgo.Message) bool
	handleCommand(command string, args []string, session *discordgo.Session, message *discordgo.Message) bool
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
