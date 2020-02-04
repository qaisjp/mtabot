package main

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
)

const modRequest = "<@%s> requested a moderator in <#%s> here: %s\nReason: %s"

func (b *bot) requestMod(m *discordgo.Message, parts []string) {
	url := composeMessageURL(m)

	message := "No message provided."
	if len(parts) > 0 {
		message = strings.Join(parts, " ")
	}
	b.discord.ChannelMessageSend(feedChannel, fmt.Sprintf(modRequest, m.Author.ID, m.ChannelID, url, message))
}
