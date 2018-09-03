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
			{RemoveFromCategory, "Remove a command from a category. Arguments: *<command>*"},
			{DeleteCategory, "Delete an existing category. Commands in the category will not be removed. Arguments: *<category>*"},
			{ListCommands, "List existing custom commands and categories."},
		},
		nil, true)
}

func (*custom) handlePrefix(string, *dispatch.Message) bool {
	return false
}

func (c *custom) handleCommand(m *dispatch.Message) bool {
	switch m.Command {
	case AddCommand:
		addCommand(m)
	case RemoveCommand:
		removeCommand(m)
		break
	case EditCommand:
		editCommand(m)
		break
	case SetHelpText:
		setHelpText(m)
		break
	case AddToCategory:
		addToCategory(m)
		break
	case RemoveFromCategory:
		removeFromCategory(m)
		break
	case DeleteCategory:
		deleteCategory(m)
		break
	case ListCommands:
		listCommands(m)
	default:
		return false
	}
	return true
}

func getCommandText(m *dispatch.Message) *string {
	if m.RawArgs == nil {
		m.ReplyToChannel("Missing command alias text.")
		return nil
	}
	text := strings.Join(m.RawArgs[1:], " ")
	return &text
}

func deleteCategory(m *dispatch.Message) {
	if len(m.Args) != 1 {
		m.ReplyToChannel("**Error:** Invalid syntax. Expected: <category>")
		return
	}
	catName := m.Args[0]
	cat := database.FetchCommandGroup(catName)
	if cat == nil {
		m.ReplyToChannel("**Error:** No category named **%s** found.", catName)
		return
	}

	database.UpdateCommandAlias(database.GroupIdField, cat.Id, database.GroupIdField, nil)
	if !database.RemoveCommandGroup(catName) {
		m.ReplyToChannel("**Error:** Failed to remove command group %s.", catName)
		return
	}
	m.ReplyToChannel("Removed command group %s.", catName)
}

func addToCategory(m *dispatch.Message) {
	if len(m.Args) < 2 {
		m.ReplyToChannel("**Error:** Invalid syntax. Expected: <category> <command>")
		return
	}

	if !database.HasCommandAlias(m.Args[1]) {
		m.ReplyToChannel("Command **%s** doesn't exist.", m.Args[1])
		return
	}

	if database.HasCommandAlias(m.Args[0]) || dispatch.Dispatcher.HasCommand(m.Args[0]) {
		m.ReplyToChannel("Error: Cannot add category **%s** since there's already a command with that name.", m.Args[0])
		return
	}

	categoryObj := database.FetchOrCreateCommandGroup(m.Args[0])
	if categoryObj == nil {
		// Make new category.
		m.ReplyToChannel("Internal Error: Unable to load or create category **%s**.", m.Args[0])
		return
	}
	commands := categoryObj.FetchCommands()
	for _, cmdObj := range commands {
		if cmdObj.Command == m.Args[1] {
			m.ReplyToChannel("Command **%s** already in category **%s**.", m.Args[1], m.Args[0])
			return
		}
	}
	if !database.UpdateCommandAlias(database.CommandField, m.Args[1], database.GroupIdField, categoryObj.Id) {
		m.ReplyToChannel("Internal Error: Failed to add command **%s** to category **%s**.", m.Args[1], m.Args[0])
	}
	m.ReplyToChannel("Command **%s** added to category **%s**.", m.Args[1], m.Args[0])
}

func removeFromCategory(m *dispatch.Message) {
	if len(m.Args) != 1 {
		m.ReplyToChannel("**Error:** Invalid syntax. Expected: <command>")
		return
	}
	cmdName := m.Args[0]
	cmd := database.FetchCommandAlias(cmdName)
	if cmd == nil {
		m.ReplyToChannel("No command named **%s** found.", cmdName)
		return
	}
	if cmd.GroupId == nil {
		m.ReplyToChannel("**%s** is not part of a category.", cmdName)
		return
	}
	if !database.UpdateCommandAlias(database.CommandField, cmdName, database.GroupIdField, nil) {
		m.ReplyToChannel("Failed to remove category from command **%s**.", cmdName)
		return
	}
	m.ReplyToChannel("**%s** removed from category successfully.", cmdName)
}

func addCommand(m *dispatch.Message) {
	if len(m.Args) < 2 {
		m.ReplyToChannel("**Error:** Invalid syntax. Expected: <command> <new text>")
		return
	}
	cmd := m.Args[0]
	if dispatch.Dispatcher.HasCommand(cmd) {
		m.ReplyToChannel("**Error:** Command **%s** is a predefined command. Pick another name.", cmd)
		return
	}
	if database.HasCommandAlias(cmd) {
		m.ReplyToChannel("**Error:** Command **%s** already exists. Use `%s%s` instead.", cmd,
			core.Settings.CommandPrefix(), EditCommand)
		return
	}
	if database.HasCommandGroup(cmd) {
		m.ReplyToChannel("**Error:** Cannot add command **%s** since there's already a category with that name.", cmd)
		return
	}

	if commandText := getCommandText(m); commandText != nil {
		ok := database.CreateCommandAlias(cmd, *commandText)
		if ok {
			core.LogInfoF("%s added command alias %s.", m.Author.Username, cmd)
			m.ReplyToChannel("Command alias for **%s** created successfully.", cmd)
			return
		}
	}
	core.LogDebug("Command was not created.")
	m.ReplyToChannel("Internal error. Unable to create command alias.")
}

func setHelpText(m *dispatch.Message) {
	var cmd string
	var helpText *string

	switch len(m.Args) {
	case 0:
		m.ReplyToChannel("**Error:** Invalid syntax. Expected: <command> [optional new help text]")
	default:
		helpText = getCommandText(m)
		fallthrough
	case 1:
		cmd = m.Args[0]
	}
	if database.HasCommandAlias(cmd) {
		if database.UpdateCommandAlias(database.CommandField, cmd, database.HelpField, helpText) {
			core.LogInfoF("%s updated help text for command %s.", m.Author.Username, cmd)
			m.ReplyToChannel("Help text for command %s was updated.", cmd)
		} else {
			core.LogDebug("Command was not updated.")
			m.ReplyToChannel("Internal error. Unable to update command alias.")
		}
	} else if database.HasCommandGroup(cmd) {
		if database.UpdateCommandGroup(database.CommandField, cmd, database.HelpField, helpText) {
			core.LogInfoF("%s updated help text for command group %s.", m.Author.Username, cmd)
			m.ReplyToChannel("Help text for command group %s was updated.", cmd)
		} else {
			core.LogDebug("Command group was not updated.")
			m.ReplyToChannel("Internal error. Unable to update command group.")
		}
	} else {
		m.ReplyToChannel("**Error:** No command or command group **%s** found.", cmd)
	}
}

func editCommand(m *dispatch.Message) {
	if len(m.Args) < 2 {
		m.ReplyToChannel("**Error:** Invalid syntax. Expected: <command> <new text>")
		return
	}
	cmd := m.Args[0]
	if !database.HasCommandAlias(cmd) {
		m.ReplyToChannel("**Error:** Command **%s** doesn't exist. Use `%s%s` instead.", cmd,
			core.Settings.CommandPrefix(), AddCommand)
		return
	}
	if commandText := getCommandText(m); commandText != nil {
		ok := database.UpdateCommandAlias(database.CommandField, cmd, database.ValueField, commandText)
		if ok {
			core.LogInfoF("%s updated command alias %s.", m.Author.Username, cmd)
			m.ReplyToChannel("Command alias for **%s** updated successfully.", cmd)
			return
		}
	}
	core.LogDebug("Command was not updated.")
	m.ReplyToChannel("Internal error. Unable to update command alias.")
}

func removeCommand(m *dispatch.Message) {
	if len(m.Args) == 0 {
		m.ReplyToChannel("**Error:** Invalid syntax. Expected: <command>.")
		return
	}
	cmd := m.Args[0]

	if !database.RemoveCommandAlias(cmd) {
		m.ReplyToChannel("**Error:** Command **%s** doesn't exist.")
		return
	}
	core.LogInfoF("%s removed command alias %s.", m.Author.Username, cmd)
	m.ReplyToChannel("Command %s removed.", cmd)
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
	output = append(output, fmt.Sprint("Category **", grp.Command, "**: "))
	if grp.Help != nil && len(*grp.Help) > 0 {
		output[0] = fmt.Sprint(output[0], *grp.Help)
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
