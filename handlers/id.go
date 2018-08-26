package handlers

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/neotron/GoBot/dispatch"
	"github.com/thoas/go-funk"
)

type ident struct {
	dispatch.MessageHandler
}

func init() {
	dispatch.Register(&ident{dispatch.MessageHandler{}},
		[]dispatch.MessageCommand{
			{"id", "Return Discord ID for the user, or all @mentioned users"},
		},
		nil, false)
}

func (*ident) handleCommand(command string, args []string, session *discordgo.Session, message *discordgo.Message) bool {
	var identities []string
	addAuthor := func(user *discordgo.User) {
		identities = append(identities, fmt.Sprintf("%v has id %s", user.Username, user.ID))
	}
	if len(args) == 0 {
		addAuthor(message.Author)
	} else {
		funk.ForEach(message.Mentions, addAuthor)
	}
	if len(identities) > 0 {
		session.ChannelMessageSend(message.ChannelID, fmt.Sprintf("Identities:\n\t%s", strings.Join(identities, "\n\t")))
	} else {
		session.ChannelMessageSend(message.ChannelID, "No one was identified")
	}
	return true
}
