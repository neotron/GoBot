package handlers

import (
	"fmt"
	"strings"

	"GoBot/core/dispatch"

	"github.com/bwmarrin/discordgo"
	"github.com/thoas/go-funk"
)

type ident struct {
	dispatch.NoOpMessageHandler
}

func init() {
	dispatch.Register(&ident{},
		[]dispatch.MessageCommand{
			{"id", "Return Discord ID for the user, or all @mentioned users"},
			{"cid", "Return Discord channel id for the current channel."},
		},
		nil, false)
}

func (*ident) HandleCommand(m *dispatch.Message) bool {
	switch m.Command {
	case "id":
		var identities []string
		addAuthor := func(user *discordgo.User) {
			identities = append(identities, fmt.Sprintf("%v has id %s", user.Username, user.ID))
		}
		if len(m.Args) == 0 {
			addAuthor(m.Message.Author)
		} else {
			funk.ForEach(m.Message.Mentions, addAuthor)
		}
		if len(identities) > 0 {
			m.ReplyToChannel("Identities:\n\t%s", strings.Join(identities, "\n\t"))
		} else {
			m.ReplyToChannel("No one was identified")
		}
		break
	case "cid":
		m.ReplyToChannel("Channel id is %s.", m.ChannelID);
		break;

	}
	return true
}
