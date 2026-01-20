package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"GoBot/core"
	"GoBot/core/database"
	_ "GoBot/core/database" // Initialize database
	"GoBot/core/dispatch"
	"GoBot/core/dispatch/handlers"
	_ "GoBot/core/dispatch/handlers" // Load the handlers to let them self-register
	"GoBot/core/services"

	"github.com/bwmarrin/discordgo"
)

// Variables used for command line parameters
var (
	settingsFile string
)

func init() {

	flag.StringVar(&settingsFile, "c", "config-dev.json", "Configuration path")
	flag.Parse()
}

func main() {
	core.LoadSettings(settingsFile)
	database.InitalizeDatabase()
	defer database.Close()
	dispatch.SettingsLoaded()

	// Start EDDN listener for carrier location updates
	services.StartEDDNListener()

	// Create a new Discord session using the provided bot token.
	dg, err := discordgo.New("Bot " + core.Settings.AuthToken())
	if err != nil {
		core.LogFatal("error creating Discord session,", err)
		return
	}

	// Register handlers
	dg.AddHandler(messageCreate)
	dg.AddHandler(messageUpdate)
	dg.AddHandler(interactionCreate)

	// Open a websocket connection to Discord and begin listening.
	err = dg.Open()
	if err != nil {
		core.LogFatal("error opening connection,", err)
		return
	}

	defer dg.Close()

	// Set Discord session for services (flight log posting)
	services.SetDiscordSession(dg)

	// Process carrier update channel messages on startup
	services.ProcessCarrierUpdateChannelOnStartup(dg)

	// Register slash commands after connection is open
	handlers.RegisterCarrierSlashCommands(dg)
	handlers.RegisterEliteDangerousSlashCommands(dg)

	// Wait here until CTRL-C or other term signal is received.
	core.LogInfoF("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc
}

// This function will be called (due to AddHandler above) every time a new
// message is created on any channel that the autenticated bot has access to.
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	go dispatch.Dispatch(s, m.Message)

	// Process carrier update channel messages (new messages and edits)
	if m.Author != nil && m.Content != "" {
		go services.ProcessCarrierUpdateMessage(m.Author.ID, m.ChannelID, m.Content)
	}
}

func messageUpdate(s *discordgo.Session, m *discordgo.MessageUpdate) {
	go dispatch.Dispatch(s, m.Message)

	// Process carrier update channel messages
	if m.Author != nil && m.Content != "" {
		go services.ProcessCarrierUpdateMessage(m.Author.ID, m.ChannelID, m.Content)
	}
}

func interactionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Try each handler until one handles the interaction
	if handlers.HandleEliteDangerousSlashCommand(s, i) {
		return
	}
	handlers.HandleCarrierSlashCommand(s, i)
}
