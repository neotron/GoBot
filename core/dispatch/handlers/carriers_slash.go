package handlers

import (
	"strconv"
	"strings"

	"GoBot/core"
	"GoBot/core/services"

	"github.com/bwmarrin/discordgo"
)

var carrierSlashCommands = []*discordgo.ApplicationCommand{
	{
		Name:        "carriers",
		Description: "List all fleet carriers with current status",
	},
	{
		Name:        "carrierjump",
		Description: "Set carrier jump time",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "carrier",
				Description:  "Carrier station ID",
				Required:     true,
				Autocomplete: true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "time",
				Description: "Jump time (e.g., '20th January, 18:30 UTC' or unix timestamp)",
				Required:    true,
			},
		},
	},
	{
		Name:        "carrierdest",
		Description: "Set carrier destination",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "carrier",
				Description:  "Carrier station ID",
				Required:     true,
				Autocomplete: true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "system",
				Description: "Destination system name",
				Required:    true,
			},
		},
	},
	{
		Name:        "carrierstatus",
		Description: "Set carrier status message",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "carrier",
				Description:  "Carrier station ID",
				Required:     true,
				Autocomplete: true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "status",
				Description: "Status message",
				Required:    true,
			},
		},
	},
	{
		Name:        "carrierclear",
		Description: "Clear carrier field",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "carrier",
				Description:  "Carrier station ID",
				Required:     true,
				Autocomplete: true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "field",
				Description: "Field to clear",
				Required:    true,
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{Name: "Jump Time", Value: "jump"},
					{Name: "Destination", Value: "dest"},
					{Name: "Status", Value: "status"},
					{Name: "All", Value: "all"},
				},
			},
		},
	},
	{
		Name:        "carrierloc",
		Description: "Set carrier location manually",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "carrier",
				Description:  "Carrier station ID",
				Required:     true,
				Autocomplete: true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "system",
				Description: "Current system name",
				Required:    true,
			},
		},
	},
}

// RegisterAllSlashCommands registers all slash commands with Discord
func RegisterAllSlashCommands(s *discordgo.Session) {
	guildId := core.Settings.SlashCommandGuildId()

	// Combine all slash commands
	allCommands := append(carrierSlashCommands, GetEliteDangerousSlashCommands()...)

	// Use bulk overwrite for efficiency (single API call instead of one per command)
	// If guildId is empty, registers globally (can take up to 1hr to propagate)
	// If guildId is set, registers to that guild only (instant)
	registered, err := s.ApplicationCommandBulkOverwrite(s.State.User.ID, guildId, allCommands)
	if err != nil {
		core.LogErrorF("Failed to register slash commands: %s", err)
		return
	}

	scope := "globally"
	if guildId != "" {
		scope = "to guild " + guildId
	}
	core.LogInfoF("Registered %d slash commands %s", len(registered), scope)
}

// HandleCarrierSlashCommand handles carrier slash command interactions
func HandleCarrierSlashCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		handleCarrierSlashAppCommand(s, i)
	case discordgo.InteractionApplicationCommandAutocomplete:
		handleCarrierAutocomplete(s, i)
	}
}

func handleCarrierSlashAppCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()

	// Get user ID (works for both guild and DM)
	var userID string
	if i.Member != nil {
		userID = i.Member.User.ID
	} else if i.User != nil {
		userID = i.User.ID
	}

	switch data.Name {
	case "carriers":
		output := services.FormatCarrierList()
		respond(s, i, output, true)

	case "carrierjump":
		if !canManageCarriersSlash(userID, i.ChannelID) {
			respond(s, i, "You don't have permission to manage carriers.", true)
			return
		}
		stationId := strings.ToUpper(data.Options[0].StringValue())
		timeInput := data.Options[1].StringValue()

		timestamp, err := services.ParseJumpTime(timeInput)
		if err != nil {
			respond(s, i, "**Error:** "+err.Error(), true)
			return
		}

		if err := services.SetCarrierJumpTime(stationId, timestamp); err != nil {
			respond(s, i, "**Error:** "+err.Error(), true)
			return
		}
		respond(s, i, formatJumpTimeResponse(stationId, timestamp), true)
		services.PostCarrierFlightLog(stationId, []string{"jump time updated"})

	case "carrierdest":
		if !canManageCarriersSlash(userID, i.ChannelID) {
			respond(s, i, "You don't have permission to manage carriers.", true)
			return
		}
		stationId := strings.ToUpper(data.Options[0].StringValue())
		destination := data.Options[1].StringValue()

		if err := services.SetCarrierDestination(stationId, destination); err != nil {
			respond(s, i, "**Error:** "+err.Error(), true)
			return
		}
		respond(s, i, formatDestinationResponse(stationId, destination), true)
		services.PostCarrierFlightLog(stationId, []string{"destination: " + destination})

	case "carrierstatus":
		if !canManageCarriersSlash(userID, i.ChannelID) {
			respond(s, i, "You don't have permission to manage carriers.", true)
			return
		}
		stationId := strings.ToUpper(data.Options[0].StringValue())
		status := data.Options[1].StringValue()

		if err := services.SetCarrierStatus(stationId, status); err != nil {
			respond(s, i, "**Error:** "+err.Error(), true)
			return
		}
		respond(s, i, formatStatusResponse(stationId, status), true)
		services.PostCarrierFlightLog(stationId, []string{"status: " + status})

	case "carrierclear":
		if !canManageCarriersSlash(userID, i.ChannelID) {
			respond(s, i, "You don't have permission to manage carriers.", true)
			return
		}
		stationId := strings.ToUpper(data.Options[0].StringValue())
		field := data.Options[1].StringValue()

		if err := services.ClearCarrierField(stationId, field); err != nil {
			respond(s, i, "**Error:** "+err.Error(), true)
			return
		}
		respond(s, i, formatClearResponse(stationId, field), true)
		if field == "all" {
			services.PostCarrierFlightLog(stationId, []string{"all fields cleared"})
		} else {
			services.PostCarrierFlightLog(stationId, []string{field + " cleared"})
		}

	case "carrierloc":
		if !canManageCarriersSlash(userID, i.ChannelID) {
			respond(s, i, "You don't have permission to manage carriers.", true)
			return
		}
		stationId := strings.ToUpper(data.Options[0].StringValue())
		system := data.Options[1].StringValue()

		if err := services.SetCarrierLocation(stationId, system); err != nil {
			respond(s, i, "**Error:** "+err.Error(), true)
			return
		}
		respond(s, i, formatLocationResponse(stationId, system), true)
		services.PostCarrierFlightLog(stationId, []string{"location: " + system})
	}
}

func handleCarrierAutocomplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()

	var choices []*discordgo.ApplicationCommandOptionChoice
	carriers := core.Settings.Carriers()

	for _, c := range carriers {
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  c.Name + " (" + c.StationId + ")",
			Value: c.StationId,
		})
	}

	// Filter based on what user has typed
	for _, opt := range data.Options {
		if opt.Focused {
			typed := strings.ToLower(opt.StringValue())
			if typed != "" {
				filtered := make([]*discordgo.ApplicationCommandOptionChoice, 0)
				for _, c := range choices {
					if strings.Contains(strings.ToLower(c.Name), typed) ||
						strings.Contains(strings.ToLower(c.Value.(string)), typed) {
						filtered = append(filtered, c)
					}
				}
				choices = filtered
			}
			break
		}
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Choices: choices,
		},
	})
}

func canManageCarriersSlash(userID, channelID string) bool {
	if channelID == SecureChannnel {
		return true
	}
	return core.Settings.IsCarrierOwner(userID)
}

func respond(s *discordgo.Session, i *discordgo.InteractionCreate, content string, ephemeral bool) {
	flags := discordgo.MessageFlags(0)
	if ephemeral {
		flags = discordgo.MessageFlagsEphemeral
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   flags,
		},
	})
}

func formatJumpTimeResponse(stationId string, timestamp int64) string {
	ts := strconv.FormatInt(timestamp, 10)
	return "Jump time for **" + stationId + "** set to <t:" + ts + ":F> (<t:" + ts + ":R>)"
}

func formatDestinationResponse(stationId, destination string) string {
	return "Destination for **" + stationId + "** set to **" + destination + "**"
}

func formatStatusResponse(stationId, status string) string {
	return "Status for **" + stationId + "** set to: " + status
}

func formatClearResponse(stationId, field string) string {
	if field == "all" {
		return "All fields cleared for **" + stationId + "**"
	}
	return "Field `" + field + "` cleared for **" + stationId + "**"
}

func formatLocationResponse(stationId, system string) string {
	return "Location for **" + stationId + "** set to **" + system + "**"
}
