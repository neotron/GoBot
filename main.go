package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"github.com/bwmarrin/discordgo"
)

// Variables used for command line parameters
var (
	settingsFile string
	dispatcher *MessageDispatcher
)

func init() {

	flag.StringVar(&settingsFile, "c", "config-dev.json", "Configuration path")
	flag.Parse()
}

func main() {
	settings := LoadSettings(settingsFile)
	// Create a new Discord session using the provided bot token.
	dg, err := discordgo.New("Bot " + settings.AuthToken())
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		return
	}

	// Register the messageCreate func as a callback for MessageCreate events.
	dg.AddHandler(messageCreate)
	dg.AddHandler(messageUpdate)

	// Open a websocket connection to Discord and begin listening.
	err = dg.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}

	// create message dispatcher
	dispatcher = CreateDispatcher(settings)

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Cleanly close down the Discord session.
	dg.Close()
}

// This function will be called (due to AddHandler above) every time a new
// message is created on any channel that the autenticated bot has access to.
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	dispatcher.Handle(s, m.Message)
}

func messageUpdate(s *discordgo.Session, m *discordgo.MessageUpdate) {
	dispatcher.Handle(s, m.Message)
}
