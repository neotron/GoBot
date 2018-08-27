package handlers

import (
	"fmt"
	"strings"

	"GoBot/core"
	"GoBot/core/database"
	"GoBot/core/dispatch"
	"github.com/thoas/go-funk"
)

type custom struct {
	dispatch.NoOpMessageHandler
}

const (
	AddCommand         = "addcmd"
	RemoveCommand      = "rmcmd"
	EditCommand        = "editcmd"
	SetHelpText        = "sethelp"
	AddToCategory      = "addtocat"
	RemoveFromCategory = "rmfromcat"
	DeleteCategory     = "delcat"
	ListCommands       = "listcmds"
	Crash              = "crash"
)

func (*custom) CommandGroup() string {
	return "Custom Command Management"
}

func init() {
	dispatch.Register(&custom{},
		[]dispatch.MessageCommand{
			{AddCommand, "Add new command. Arguments: *<command> <text>*"},
			{RemoveCommand, "Remove existing command. Arguments: *<command>*"},
			{EditCommand, "Replace text for existing command. Arguments: *<command> <new text>*"},
			{SetHelpText, "Set (or remove) a help string for an existing command or category. Arguments: *<command or category> [help text]*"},
			{AddToCategory, "Add an existing command to a category. Category will be created if it doesn't exist. Arguments: *<category> <command>*"},
			{RemoveFromCategory, "Remove a command from a category. Arguments: *<category> <command>*"},
			{DeleteCategory, "Delete an existing category. Commands in the category will not be removed. Arguments: *<category>*"},
			{ListCommands, "List existing custom commands and categories."},
			{Crash, ""},
		},
		nil, true)
}

func (*custom) handlePrefix(string, *dispatch.Message) bool {
	return false
}

func (*custom) handleCommand(m *dispatch.Message) bool {
	switch m.Command {
	case AddCommand:
		break
	case RemoveCommand:
		break
	case EditCommand:
		break
	case SetHelpText:
		break
	case AddToCategory:
		break
	case RemoveFromCategory:
		break
	case DeleteCategory:
		break
	case ListCommands:
		go listCommands(m)
	case Crash:
		break
	}
	return false
}
func listCommands(m *dispatch.Message) {
	var output []string
	prefix := core.Settings.CommandPrefix()
	if groups := database.FetchCommandGroups(); len(groups) > 0 {
		output = append(output, "**Commands by Category**:\n\t")
		groups := funk.Map(groups, func(group database.CommandGroup) string {
			cmdString := "No commands in category."
			if cmds := group.FetchCommands(); cmds != nil {
				cmdString = strings.Join(funk.Map(cmds, func(cmd database.CommandAlias) string { return cmd.Command }).([]string), ", ")
			}
			return fmt.Sprintf(" **%s%s:**\n\t%s", prefix, group.Command, cmdString)
		}).([]string)
		output = append(output, strings.Join(groups, "\n"))
	} else {
		output = append(output, "**Categories:** \n\tNone found")
	}
	if fetchedCommands := database.FetchStandaloneCommands(); len(fetchedCommands) > 0 {
		output = append(output, fmt.Sprint("\n**Uncategorised Commands:**\n\t",
			strings.Join(funk.Map(fetchedCommands, func(cmd database.CommandAlias) string {
				return cmd.Command
			}).([]string), ", ")))
	} else {
		output = append(output, "\n**Uncategorised Commands:**\n\tNone found")
	}

	outputString := strings.Join(output, "\n")
	if m.Flags.IsSet(dispatch.Here) {
		m.ReplyToChannel(outputString)
	} else {
		m.ReplyToSender(outputString)
	}
}

func (*custom) handleAnything(m *dispatch.Message) bool {
	if cmd := database.FetchCommandAlias(m.Command); cmd != nil {
		handleCommandAlias(cmd, m)
		return true
	}

	if grp := database.FetchCommandGroup(m.Command); grp != nil {
		handleCommandGroup(grp, m)
		return true
	}
	return false
}

func handleCommandGroup(grp *database.CommandGroup, m *dispatch.Message) {
	var output []string
	output = append(output, fmt.Sprint("**Category ", grp.Command, "**: "))
	if grp.Help != nil && len(*grp.Help) > 0 {
		output[0] = fmt.Sprint(output, *grp.Help)
	}
	if sortedCommands := grp.FetchCommands(); sortedCommands != nil {
		for _, command := range sortedCommands {
			var cmdline = fmt.Sprintf("\t**%s%s**", core.Settings.CommandPrefix(), command.Command)
			if command.Help != nil && len(*command.Help) > 0 {
				cmdline = fmt.Sprint(cmdline, ": ", *command.Help)
			}
			output = append(output, cmdline)
		}
	}
	m.ReplyToChannel(strings.Join(output, "\n"))
}

func handleCommandAlias(cmd *database.CommandAlias, m *dispatch.Message) {
	if m.Flags.IsSet(dispatch.Help) {
		if cmd.Help != nil && len(*cmd.Help) > 0 {
			var helpMessage = fmt.Sprintf("**%s%s**: %s", core.Settings.CommandPrefix(),
				cmd.Command, *cmd.Help)
			if cmd.Longhelp != nil && len(*cmd.Longhelp) > 0 {
				longHelpMessage := fmt.Sprint(helpMessage, "\n\n", *cmd.Longhelp)
				if cmd.PMEnabled {
					m.ReplyToSender(longHelpMessage)
					if m.IsPM {
						// Since it's a PM, no further messages are needed.
						return
					}
					helpMessage = fmt.Sprint(helpMessage, " (see pm for details)")
				} else {
					helpMessage = longHelpMessage
				}
			}
			m.ReplyToChannel(helpMessage)
		} else {
			m.ReplyToChannel("**%s%s**: No help available.", core.Settings.CommandPrefix(), cmd.Command)
		}
	} else {
		m.ReplyToChannel(cmd.Value)
	}
}
