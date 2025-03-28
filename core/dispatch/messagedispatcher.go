package dispatch

import (
	"fmt"
	"sort"
	"strings"

	"GoBot/core"
	"github.com/bwmarrin/discordgo"
	"github.com/thoas/go-funk"
)

// MessageDispatcher This class will parse and dispatch commands to the appropriate Command handler.
// It also filters out any response from messages sent by itself, and which don't have the proper
// Command prefix, as defined in the config file
type MessageDispatcher struct {
	NoOpMessageHandler
	// allows prefix handling, i.e "randomcat" and "randomdog" could both go to a "random" prefix handler
	prefixHandlers map[string][]MessageHandler
	// requires either just the Command, i.e "route" or Command with arguments "route 32 2.3"
	commandHandlers map[string][]MessageHandler
	// Anything matching
	anythingHandlers []MessageHandler
	// Command help
	commandHelp map[string]map[string][]string
}

// Dispatcher Object used for dispatching messages to the handlers.
var Dispatcher = MessageDispatcher{
	prefixHandlers:  map[string][]MessageHandler{},
	commandHandlers: map[string][]MessageHandler{},
	commandHelp:     map[string]map[string][]string{},
}

func init() {
	Register(&Dispatcher, []MessageCommand{{"help", ""}}, nil, false)
}

// Register a new command handler with zero or more commands, prefix handlers and optional wildcard matching
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

func (d *MessageDispatcher) HasCommand(cmd string) bool {
	return d.commandHandlers[cmd] != nil || d.prefixHandlers[cmd] != nil
}

// HandleCommand This handle basically deals with help
func (d *MessageDispatcher) HandleCommand(m *Message) bool {
	go func() {
		groups := funk.Keys(d.commandHelp).([]string)
		sort.Strings(groups)
		for _, group := range groups {
			var output []string
			if group != "" {
				output = append(output, fmt.Sprintf("**%s**", group))
			} else {
				output = append(output, "**General Commands**:")
			}
			commands := funk.Keys(d.commandHelp[group]).([]string)
			sort.Strings(commands)
			for _, command := range commands {
				output = append(output, strings.Join(d.commandHelp[group][command], "\n"))
			}
			//if message.flags.contains(.Here) {
			//        message.replyToChannel(output.joined(separator: "\n"));
			//    } else {
			<-m.ReplyToSender(strings.Join(output, "\n"))
			//    }
		}
	}()
	return true
}

func SettingsLoaded() {
	cache := make(map[string]bool)
	isCalled := func(h MessageHandler) bool {
		key := h.CommandGroup()
		if cache[key] {
			return true
		}
		cache[key] = true
		return false
	}
	for _, handler := range Dispatcher.anythingHandlers {
		if !isCalled(handler) {
			handler.SettingsLoaded()
		}
	}
	for _, handlers := range Dispatcher.prefixHandlers {
		for _, handler := range handlers {
			if !isCalled(handler) {
				handler.SettingsLoaded()
			}
		}
	}
	for _, handlers := range Dispatcher.commandHandlers {
		for _, handler := range handlers {
			if !isCalled(handler) {
				handler.SettingsLoaded()
			}
		}
	}
}

func Dispatch(session *discordgo.Session, message *discordgo.Message) {
	Dispatcher.Dispatch(session, message)
}

// Dispatch Parse and dispatch the message.
func (d *MessageDispatcher) Dispatch(session *discordgo.Session, message *discordgo.Message) {
	// Short-circuit if author of the message is the bot itself to avoid loops
	if message.Author == nil || message.Author.ID == session.State.User.ID {
		return
	}

	core.LogDebug("Got message: ", message.Content)

	// This handles @BotName command trimming
	trimmed := strings.TrimPrefix(message.Content, fmt.Sprintf("<@%s> ", session.State.User.ID))
	// If directly addressed, it will respond to unknown commands in a PM.
	isDirectAddressed := trimmed != message.Content
	// And this will trim the configured command prefix, which is optional if @Bot syntax is used
	trimmed = strings.TrimPrefix(trimmed, core.Settings.CommandPrefix())
	// And finally, if we didn't trim anything, check to see if it was a DM.
	var isDM bool
	if trimmed == message.Content {
		var err error
		isDM, err = comesFromDM(session, message)
		if !isDM || err != nil {
			return
		}
	}

	// Split the Command into parameters, and clean them up.
	rawArgs := strings.Split(trimmed, " ")
	args := funk.FilterString(rawArgs, func(str string) bool {
		return strings.Trim(str, "\t\r") != ""
	})

	// Just a bunch of whitespaces
	if len(args) == 0 {
		return
	}

	core.LogDebug("Parsed parameters:", args)

	command := strings.ToLower(args[0])
	args = args[1:]
	cmdMessage := &Message{
		message, session, command, args,
		rawArgs[1:], parseCommandFlags(args), isDM,
	}
	if commandHandlers := d.commandHandlers[command]; len(commandHandlers) > 0 {
		core.LogDebugF("Found %d Command handlers for %s.", len(commandHandlers), command)
		for _, handler := range commandHandlers {
			if handler.HandleCommand(cmdMessage) {
				core.LogDebug("   => handled.")
				return
			}
		}
	}

	for prefix, handlers := range d.prefixHandlers {
		if !strings.HasPrefix(command, prefix) {
			continue
		}
		suffix := strings.TrimPrefix(command, prefix)
		core.LogDebugF("Found %d prefix handlers for %s.", len(handlers), prefix)
		for _, handler := range handlers {
			if handler.HandlePrefix(prefix, suffix, cmdMessage) {
				if core.IsLogDebug() {
					core.LogDebugF("   => handled by %s.", toName(handler))
				}
				return
			}
		}
	}

	for _, handler := range d.anythingHandlers {
		if core.IsLogDebug() {
			core.LogDebugF("Trying anything handler %s...", toName(handler))
		}
		if handler.HandleAnything(cmdMessage) {
			core.LogDebug("    => handled")
			return
		}
	}

	if isDirectAddressed {
		cmdMessage.ReplyToSender("I'm not sure what you meant. You can use the %shelp command for a list of what I can do.", core.Settings.CommandPrefix())
	}
}

// Helper method to register a Command for a handler.
func (d *MessageDispatcher) addHandlerForCommand(command MessageCommand, dict *map[string][]MessageHandler, handler MessageHandler) {
	commandStr := strings.ToLower(command.Command)

	helpString, group := command.Help, handler.CommandGroup()
	if len(helpString) > 0 {
		if d.commandHelp[group] == nil {
			d.commandHelp[group] = map[string][]string{commandStr: {}}
		} else if d.commandHelp[group][commandStr] == nil {
			d.commandHelp[group][commandStr] = []string{}
		}
		d.commandHelp[group][commandStr] = append(d.commandHelp[group][commandStr], fmt.Sprint("\t**", core.Settings.CommandPrefix(), commandStr, "**: ", helpString, ""))
	}

	(*dict)[commandStr] = append((*dict)[commandStr], handler)
	if core.IsLogInfo() {
		core.LogInfoF("Registered Command: %s for %s", commandStr, toName(handler))
	}
}

func comesFromDM(s *discordgo.Session, m *discordgo.Message) (bool, error) {
	channel, err := s.State.Channel(m.ChannelID)
	if err != nil {
		if channel, err = s.Channel(m.ChannelID); err != nil {
			return false, err
		}
	}

	return channel.Type == discordgo.ChannelTypeDM, nil
}

func parseCommandFlags(args []string) Flags {
	flags := None

	if funk.Contains(args, "-v") || funk.Contains(args, "--verbose") || funk.Contains(args, "verbose") {
		flags |= Verbose
	}
	if funk.Contains(args, "-h") || funk.Contains(args, "help") {
		flags |= Help
	}
	if funk.Contains(args, "here") || funk.Contains(args, "--here") {
		flags |= Here
	}
	return flags
}
