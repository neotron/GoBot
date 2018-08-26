package dispatch

import (
	"strings"

	"GoBot/core"

	"github.com/bwmarrin/discordgo"
	"github.com/thoas/go-funk"
)

// This class will parse and dispatch commands to the appropriate Command handler.
// It also filters out any response from messages sent by itself, and which don't have the proper
// Command prefix, as defined in the config file
type MessageDispatcher struct {
	// allows prefix handling, i.e "randomcat" and "randomdog" could both go to a "random" prefix handler
	prefixHandlers map[string][]MessageHandler
	// requires either just the Command, i.e "route" or Command with arguments "route 32 2.3"
	commandHandlers map[string][]MessageHandler
	// Anything matching
	anythingHandlers []MessageHandler
}

var Dispatcher = MessageDispatcher{
	prefixHandlers:  map[string][]MessageHandler{},
	commandHandlers: map[string][]MessageHandler{},
}

func Register(handler MessageHandler, commands, prefixes []MessageCommand, wildcard bool) {
	for _, prefix := range prefixes {
		Dispatcher.addHandlerForCommand(prefix, &Dispatcher.prefixHandlers, handler)
	}

	for _, command := range commands {
		Dispatcher.addHandlerForCommand(command, &Dispatcher.commandHandlers, handler)
	}

	if wildcard {
		core.LogInfoF("Registered anything matcher: %s", toName(handler))
		Dispatcher.anythingHandlers = append(Dispatcher.anythingHandlers, handler)
	}
}

func Dispatch(session *discordgo.Session, message *discordgo.Message) {
	Dispatcher.Dispatch(session, message)
}

// Parse and dispatch the message.
func (dispatcher *MessageDispatcher) Dispatch(session *discordgo.Session, message *discordgo.Message) {
	// Short-circuit if author of the message is the bot itself to avoid loops
	if message.Author == nil || message.Author.ID == session.State.User.ID {
		return
	}

	core.LogDebug("Got message: ", message.Content)

	// Ensure that the string has the prefix we're programmed to listen to
	trimmed := strings.TrimPrefix(message.Content, core.Settings.CommandPrefix())
	if trimmed == message.Content {
		return
	}

	// Split the Command into parameters, and clean them up.
	args := funk.FilterString(strings.Split(trimmed, " "), func(str string) bool {
		return strings.Trim(str, "\t\r") != ""
	})

	// Just a bunch of whitespaces
	if len(args) == 0 {
		return
	}

	core.LogDebug("Parsed parameters:", args)

	command := strings.ToLower(args[0])
	args = args[1:]
	cmdMessage := &Message{message, session, command, args}
	if commandHandlers := dispatcher.commandHandlers[command]; len(commandHandlers) > 0 {
		core.LogDebugF("Found %d Command handlers for %s.", len(commandHandlers), command)
		for _, handler := range commandHandlers {
			if handler.handleCommand(cmdMessage) {
				core.LogDebug("   => handled.")
				return
			}
		}
	}

	for prefix, handlers := range dispatcher.prefixHandlers {
		if !strings.HasPrefix(command, prefix) {
			continue
		}
		core.LogDebugF("Found %d prefix handlers for %s.", len(handlers), prefix)
		for _, handler := range handlers {
			if handler.handlePrefix(prefix, cmdMessage) {
				if core.IsLogDebug() {
					core.LogDebugF("   => handled by %s.", toName(handler))
				}
				return
			}
		}
	}

	for _, handler := range dispatcher.anythingHandlers {
		if core.IsLogDebug() {
			core.LogDebugF("Trying anything handler %s...", toName(handler))
		}
		if handler.handleAnything(cmdMessage) {
			core.LogDebug("    => handled")
			return
		}
	}
}

// Helper method to register a Command for a handler.
func (dispatcher *MessageDispatcher) addHandlerForCommand(command MessageCommand, dict *map[string][]MessageHandler, handler MessageHandler) {
	commandStr := strings.ToLower(command.Command)

	// TODO: Help Strings
	//if helpString := Command.Help, let group = handler.commandGroup {
	//    if commandHelp[group] == nil {
	//        commandHelp[group] = [commandStr: []]
	//    } else if commandHelp[group]![commandStr] == nil {
	//        commandHelp[group]![commandStr] = [String]()
	//    }
	//    commandHelp[group]?[commandStr]?.append("\t**\(Config.commandPrefix)\(commandStr)**: \(helpString)")
	//}

	(*dict)[commandStr] = append((*dict)[commandStr], handler)
	if core.IsLogInfo() {
		core.LogInfoF("Registered Command: %s for %s", commandStr, toName(handler))
	}
}
