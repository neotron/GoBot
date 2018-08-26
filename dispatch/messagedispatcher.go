package dispatch

import (
	"fmt"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/neotron/GoBot/core"
	"github.com/thoas/go-funk"
)

// This class will parse and dispatch commands to the appropriate command handler.
// It also filters out any response from messages sent by itself, and which don't have the proper
// command prefix, as defined in the config file
type MessageDispatcher struct {
	// allows prefix handling, i.e "randomcat" and "randomdog" could both go to a "random" prefix handler
	prefixHandlers map[string][]IMessageHandler
	// requires either just the command, i.e "route" or command with arguments "route 32 2.3"
	commandHandlers map[string][]IMessageHandler
	// Anything matching
	anythingHandlers []IMessageHandler
}

var Dispatcher = MessageDispatcher{
	prefixHandlers:   map[string][]IMessageHandler{},
	commandHandlers:  map[string][]IMessageHandler{},
	anythingHandlers: []IMessageHandler{},
}

func Register(handler IMessageHandler, commands, prefixes []MessageCommand, wildcard bool) {
	funk.ForEach(prefixes, func(prefix MessageCommand) {
		Dispatcher.addHandlerForCommand(prefix, &Dispatcher.prefixHandlers, handler)
	})

	funk.ForEach(commands, func(cmd MessageCommand) {
		Dispatcher.addHandlerForCommand(cmd, &Dispatcher.commandHandlers, handler)
	})

	if wildcard {
		_ = append(Dispatcher.anythingHandlers, handler)
	}
}

func Dispatch(session *discordgo.Session, message *discordgo.Message) {
	Dispatcher.Dispatch(session, message)
}

// Parse and dispatch the message.
func (dispatcher *MessageDispatcher) Dispatch(session *discordgo.Session, message *discordgo.Message) {
	fmt.Println("Got message: ", message.Content)

	// Short-circuit if author of the message is the bot itself to avoid loops
	if message.Author.ID == session.State.User.ID {
		return
	}

	// Ensure that the string has the prefix we're programmed to listen to
	trimmed := strings.TrimPrefix(message.Content, core.Settings.CommandPrefix())
	if trimmed == message.Content {
		return
	}

	// Split the command into parameters, and clean them up.
	args := funk.FilterString(strings.Split(trimmed, " "), func(str string) bool {
		return strings.Trim(str, "\t\r") != ""
	})

	// Just a bunch of whitespaces
	if len(args) == 0 {
		return
	}

	fmt.Println("Parsed parameters:", args)

	command := strings.ToLower(args[0])
	args = args[1:]

	if commandHandlers := dispatcher.commandHandlers[command]; commandHandlers != nil {
		log.Println("Found command handlers for", command)
		funk.ForEach(commandHandlers, func(handler IMessageHandler) {
			if handler.handleCommand(command, args, session, message) {
				log.Println("   => handled.")
				return
			}
		})
	}

	funk.ForEach(dispatcher.prefixHandlers, func(prefix string, handlers []IMessageHandler) {
		if strings.HasPrefix(command, prefix) {
			log.Println("Found prefix handlers for", prefix)
			funk.ForEach(handlers, func(handler IMessageHandler) {
				if handler.handlePrefix(prefix, command, args, session, message) {
					log.Println("   => handled.")
					return
				}
			})
		}
	})
	//
	//for handler in anythingHandlers {
	//    LOG_DEBUG("Trying anything handler \(handler)...")
	//    if handler.handleAnything(command, args: args, message: messageWrapper) {
	//        LOG_DEBUG("    => handled")
	//        return
	//    }
	//
	//}
}

// Helper method to register a command for a handler.
func (dispatcher *MessageDispatcher) addHandlerForCommand(command MessageCommand, dict *map[string][]IMessageHandler, handler IMessageHandler) {
	commandStr := strings.ToLower(command.Command)

	// TODO: Help Strings
	//if helpString := command.Help, let group = handler.commandGroup {
	//    if commandHelp[group] == nil {
	//        commandHelp[group] = [commandStr: []]
	//    } else if commandHelp[group]![commandStr] == nil {
	//        commandHelp[group]![commandStr] = [String]()
	//    }
	//    commandHelp[group]?[commandStr]?.append("\t**\(Config.commandPrefix)\(commandStr)**: \(helpString)")
	//}

	if (*dict)[commandStr] == nil {
		(*dict)[commandStr] = []IMessageHandler{handler}
	} else {
		(*dict)[commandStr] = append((*dict)[commandStr], handler)
	}

	log.Printf("Registered command %s.", commandStr)
}
