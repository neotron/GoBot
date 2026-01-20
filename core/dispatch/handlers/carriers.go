package handlers

import (
	"strings"

	"GoBot/core"
	"GoBot/core/dispatch"
	"GoBot/core/services"
)

type carriers struct {
	dispatch.NoOpMessageHandler
}

const (
	CarrierJump   = "carrierjump"
	CarrierDest   = "carrierdest"
	CarrierStatus = "carrierstatus"
	CarrierClear  = "carrierclear"
	CarrierLoc    = "carrierloc"
	CarriersList  = "carriers"
)

func (*carriers) CommandGroup() string {
	return "Fleet Carriers"
}

func init() {
	dispatch.Register(&carriers{},
		[]dispatch.MessageCommand{
			{CarrierJump, "Set carrier jump time. Arguments: *<station-id> <time>* (e.g., '20th January, 18:30 UTC')"},
			{CarrierDest, "Set carrier destination. Arguments: *<station-id> <system name>*"},
			{CarrierStatus, "Set carrier status. Arguments: *<station-id> <status text>*"},
			{CarrierClear, "Clear carrier field. Arguments: *<station-id> <jump|dest|status|all>*"},
			{CarrierLoc, "Set carrier location manually. Arguments: *<station-id> <system name>*"},
			{CarriersList, "List all fleet carriers with current status."},
		},
		nil, false)
}

func (c *carriers) HandleCommand(m *dispatch.Message) bool {
	switch m.Command {
	case CarriersList:
		handleCarriersList(m)
		return true
	case CarrierJump, CarrierDest, CarrierStatus, CarrierClear, CarrierLoc:
		return handleCarrierManagement(m)
	default:
		return false
	}
}

// canManageCarriers checks if user has permission to manage carriers
func canManageCarriers(m *dispatch.Message) bool {
	// Secure channel always allowed
	if m.ChannelID == SecureChannnel {
		return true
	}
	// Check if user is a carrier owner
	return core.Settings.IsCarrierOwner(m.Author.ID)
}

func handleCarrierManagement(m *dispatch.Message) bool {
	if !canManageCarriers(m) {
		m.ReplyToChannel("You don't have permission to manage carriers.")
		return true
	}

	if len(m.Args) < 1 {
		m.ReplyToChannel("**Error:** Missing station ID. Usage: `%s%s <station-id> ...`",
			core.Settings.CommandPrefix(), m.Command)
		return true
	}

	stationId := strings.ToUpper(m.Args[0])

	// Validate station ID exists
	if core.Settings.GetCarrierByStationId(stationId) == nil {
		validIds := services.GetCarrierStationIds()
		m.ReplyToChannel("**Error:** Carrier `%s` not found. Valid carriers: %s",
			stationId, strings.Join(validIds, ", "))
		return true
	}

	switch m.Command {
	case CarrierJump:
		handleSetJumpTime(m, stationId)
	case CarrierDest:
		handleSetDestination(m, stationId)
	case CarrierStatus:
		handleSetStatus(m, stationId)
	case CarrierClear:
		handleClearField(m, stationId)
	case CarrierLoc:
		handleSetLocation(m, stationId)
	}
	return true
}

func handleSetJumpTime(m *dispatch.Message, stationId string) {
	if len(m.Args) < 2 {
		m.ReplyToChannel("**Error:** Missing time. Usage: `%s%s %s <time>`\nExamples: `20th January, 18:30 UTC` or unix timestamp",
			core.Settings.CommandPrefix(), CarrierJump, stationId)
		return
	}

	// Join all args after station ID to support spaces in time format
	timeInput := strings.Join(m.Args[1:], " ")
	timestamp, err := services.ParseJumpTime(timeInput)
	if err != nil {
		m.ReplyToChannel("**Error:** %s", err)
		return
	}

	if err := services.SetCarrierJumpTime(stationId, timestamp); err != nil {
		m.ReplyToChannel("**Error:** %s", err)
		return
	}

	m.ReplyToChannel("Jump time for **%s** set to <t:%d:F> (<t:%d:R>)", stationId, timestamp, timestamp)
	services.PostCarrierFlightLog(stationId, []string{"jump time updated"})
}

func handleSetDestination(m *dispatch.Message, stationId string) {
	if len(m.Args) < 2 {
		m.ReplyToChannel("**Error:** Missing destination. Usage: `%s%s %s <system name>`",
			core.Settings.CommandPrefix(), CarrierDest, stationId)
		return
	}

	destination := strings.Join(m.Args[1:], " ")
	if err := services.SetCarrierDestination(stationId, destination); err != nil {
		m.ReplyToChannel("**Error:** %s", err)
		return
	}

	m.ReplyToChannel("Destination for **%s** set to **%s**", stationId, destination)
	services.PostCarrierFlightLog(stationId, []string{"destination: " + destination})
}

func handleSetStatus(m *dispatch.Message, stationId string) {
	if len(m.Args) < 2 {
		m.ReplyToChannel("**Error:** Missing status. Usage: `%s%s %s <status text>`",
			core.Settings.CommandPrefix(), CarrierStatus, stationId)
		return
	}

	status := strings.Join(m.Args[1:], " ")
	if err := services.SetCarrierStatus(stationId, status); err != nil {
		m.ReplyToChannel("**Error:** %s", err)
		return
	}

	m.ReplyToChannel("Status for **%s** set to: %s", stationId, status)
	services.PostCarrierFlightLog(stationId, []string{"status: " + status})
}

func handleClearField(m *dispatch.Message, stationId string) {
	if len(m.Args) < 2 {
		m.ReplyToChannel("**Error:** Missing field. Usage: `%s%s %s <jump|dest|status|all>`",
			core.Settings.CommandPrefix(), CarrierClear, stationId)
		return
	}

	field := strings.ToLower(m.Args[1])
	if err := services.ClearCarrierField(stationId, field); err != nil {
		m.ReplyToChannel("**Error:** %s", err)
		return
	}

	if field == "all" {
		m.ReplyToChannel("All fields cleared for **%s**", stationId)
		services.PostCarrierFlightLog(stationId, []string{"all fields cleared"})
	} else {
		m.ReplyToChannel("Field `%s` cleared for **%s**", field, stationId)
		services.PostCarrierFlightLog(stationId, []string{field + " cleared"})
	}
}

func handleSetLocation(m *dispatch.Message, stationId string) {
	if len(m.Args) < 2 {
		m.ReplyToChannel("**Error:** Missing system. Usage: `%s%s %s <system name>`",
			core.Settings.CommandPrefix(), CarrierLoc, stationId)
		return
	}

	system := strings.Join(m.Args[1:], " ")
	if err := services.SetCarrierLocation(stationId, system); err != nil {
		m.ReplyToChannel("**Error:** %s", err)
		return
	}

	m.ReplyToChannel("Location for **%s** set to **%s**", stationId, system)
	services.PostCarrierFlightLog(stationId, []string{"location: " + system})
}

func handleCarriersList(m *dispatch.Message) {
	output := services.FormatCarrierList()

	// Reply in channel if bot channel, otherwise DM
	if core.Settings.IsBotChannel(m.ChannelID) {
		m.ReplyToChannel("%s", output)
	} else {
		m.ReplyToSender("%s", output)
	}
}
