package handlers

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"GoBot/core"
	"GoBot/core/database"
	"GoBot/core/services"

	"github.com/bwmarrin/discordgo"
)

var (
	permissionAdministrator = int64(discordgo.PermissionAdministrator)
)

var carrierSlashCommands = []*discordgo.ApplicationCommand{
	{
		Name:        "carriers",
		Description: "List all fleet carriers with current status",
	},
	{
		Name:                     "carrierjump",
		Description:              "Set carrier jump time",
		DefaultMemberPermissions: &permissionAdministrator,
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
		Name:                     "carrierdest",
		Description:              "Set carrier destination",
		DefaultMemberPermissions: &permissionAdministrator,
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
		Name:                     "carrierstatus",
		Description:              "Set carrier status message",
		DefaultMemberPermissions: &permissionAdministrator,
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
		Name:                     "carrierclear",
		Description:              "Clear carrier field",
		DefaultMemberPermissions: &permissionAdministrator,
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
		Name:                     "carrierloc",
		Description:              "Set carrier location manually",
		DefaultMemberPermissions: &permissionAdministrator,
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
	{
		Name:                     "followers",
		Description:              "List carriers following our fleet",
		DefaultMemberPermissions: &permissionAdministrator,
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "sort",
				Description: "Sort by field",
				Required:    false,
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{Name: "Most Recent", Value: "recent"},
					{Name: "Most Sightings", Value: "times"},
					{Name: "Closest Distance", Value: "distance"},
				},
			},
		},
	},
	{
		Name:                     "followerinfo",
		Description:              "Get detailed info about a follower carrier",
		DefaultMemberPermissions: &permissionAdministrator,
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "carrier",
				Description: "Carrier station ID (e.g., ABC-123)",
				Required:    true,
			},
		},
	},
	{
		Name:        "carrierinfo",
		Description: "Get detailed info and stats about a fleet carrier",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "carrier",
				Description:  "Carrier station ID",
				Required:     true,
				Autocomplete: true,
			},
		},
	},
	{
		Name:        "carrieralert",
		Description: "Get a DM when any fleet carrier jumps near a system",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "system",
				Description: "Target system name",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionNumber,
				Name:        "distance",
				Description: "Alert distance in light years",
				Required:    true,
			},
		},
	},
	{
		Name:        "carrieralerts",
		Description: "List your active carrier proximity alerts",
	},
	{
		Name:        "carrieralertclear",
		Description: "Remove a carrier proximity alert (or all)",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "id",
				Description: "Alert ID to remove (omit to clear all)",
				Required:    false,
			},
		},
	},
}

// RegisterAllSlashCommands registers all slash commands with Discord
func RegisterAllSlashCommands(s *discordgo.Session) {
	guildId := core.Settings.SlashCommandGuildId()

	// Only register to a specific guild, not globally
	if guildId == "" {
		core.LogInfo("slashCommandGuildId not set, skipping slash command registration")
		return
	}

	// Combine all slash commands
	allCommands := append(carrierSlashCommands, GetEliteDangerousSlashCommands()...)

	// Filter commands if allowlist is configured
	allowlist := core.Settings.SlashCommandAllowlist()
	if len(allowlist) > 0 {
		allowlistMap := make(map[string]bool)
		for _, name := range allowlist {
			allowlistMap[name] = true
		}
		filtered := make([]*discordgo.ApplicationCommand, 0)
		for _, cmd := range allCommands {
			if allowlistMap[cmd.Name] {
				filtered = append(filtered, cmd)
			}
		}
		core.LogInfoF("Slash command allowlist active: %v (registering %d of %d commands)", allowlist, len(filtered), len(allCommands))
		allCommands = filtered
	}

	// Use bulk overwrite for efficiency (single API call instead of one per command)
	registered, err := s.ApplicationCommandBulkOverwrite(s.State.User.ID, guildId, allCommands)
	if err != nil {
		core.LogErrorF("Failed to register slash commands: %s", err)
		return
	}

	core.LogInfoF("Registered %d slash commands to guild %s", len(registered), guildId)
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

	case "followers":
		if !canManageCarriersSlash(userID, i.ChannelID) {
			respond(s, i, "You don't have permission to view followers.", true)
			return
		}
		sortBy := "recent" // default
		if len(data.Options) > 0 {
			sortBy = data.Options[0].StringValue()
		}
		followers := services.GetRecentFollowers(sortBy)
		output := services.FormatFollowerList(followers, sortBy)
		respond(s, i, output, true)

	case "followerinfo":
		if !canManageCarriersSlash(userID, i.ChannelID) {
			respond(s, i, "You don't have permission to view follower info.", true)
			return
		}
		stationId := strings.ToUpper(data.Options[0].StringValue())
		follower := services.GetFollowerInfo(stationId)
		output := services.FormatFollowerInfo(follower)
		respond(s, i, output, true)

	case "carrierinfo":
		stationId := strings.ToUpper(data.Options[0].StringValue())
		output := services.FormatCarrierInfo(stationId)
		respond(s, i, output, true)

	case "carrieralert":
		systemName := data.Options[0].StringValue()
		distance := data.Options[1].FloatValue()

		if distance <= 0 {
			respond(s, i, "**Error:** Distance must be greater than 0.", true)
			return
		}

		// Validate system exists in EDSM
		coords, err := services.GetSystemCoords(systemName)
		if err != nil || coords == nil {
			respond(s, i, fmt.Sprintf("**Error:** System **%s** not found in EDSM.", systemName), true)
			return
		}

		alertID, err := database.CreateProximityAlert(userID, systemName, distance)
		if err != nil {
			respond(s, i, "**Error:** Failed to create alert: "+err.Error(), true)
			return
		}
		respond(s, i, fmt.Sprintf("Proximity alert #%d created: you'll be DM'd when any fleet carrier jumps within **%.1f ly** of **%s**.", alertID, distance, systemName), true)

	case "carrieralerts":
		alerts := database.FetchProximityAlertsByUser(userID)
		if len(alerts) == 0 {
			respond(s, i, "You have no active proximity alerts.", true)
			return
		}
		var sb strings.Builder
		sb.WriteString("**Your Proximity Alerts:**\n")
		for _, a := range alerts {
			created := time.Unix(a.CreatedAt, 0).UTC().Format("2006-01-02 15:04 UTC")
			sb.WriteString(fmt.Sprintf("• **#%d** — %s (within %.1f ly) — created %s\n", a.ID, a.SystemName, a.DistanceLY, created))
		}
		respond(s, i, sb.String(), true)

	case "carrieralertclear":
		if len(data.Options) > 0 {
			alertID := data.Options[0].IntValue()
			if database.DeleteProximityAlert(alertID, userID) {
				respond(s, i, fmt.Sprintf("Proximity alert #%d removed.", alertID), true)
			} else {
				respond(s, i, fmt.Sprintf("**Error:** Alert #%d not found or not yours.", alertID), true)
			}
		} else {
			count := database.DeleteAllProximityAlerts(userID)
			if count == 0 {
				respond(s, i, "You have no proximity alerts to clear.", true)
			} else {
				respond(s, i, fmt.Sprintf("Cleared %d proximity alert(s).", count), true)
			}
		}
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
