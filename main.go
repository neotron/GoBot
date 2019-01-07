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
	_ "GoBot/core/dispatch/handlers" // Load the handlers to let them self-register

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

	// Create a new Discord session using the provided bot token.
	dg, err := discordgo.New("Bot " + core.Settings.AuthToken())
	if err != nil {
		core.LogFatal("error creating Discord session,", err)
		return
	}

	// Register the messageCreate func as a callback for MessageCreate events.
	dg.AddHandler(messageCreate)
	dg.AddHandler(messageUpdate)

	// Open a websocket connection to Discord and begin listening.
	err = dg.Open()
	if err != nil {
		core.LogFatal("error opening connection,", err)
		return
	}

	defer dg.Close()

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
}

func messageUpdate(s *discordgo.Session, m *discordgo.MessageUpdate) {
	go dispatch.Dispatch(s, m.Message)
}
