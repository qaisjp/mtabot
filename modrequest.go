package main

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
)

const modRequest = "<@%s> requested a moderator in <#%s> here: %s\nReason: %s"
const requestRecv = "Your request was received <@%s>, a moderator will review and report back soon."

func (b *bot) requestMod(m *discordgo.Message, parts []string) {
	url := composeMessageURL(m)

	message := "No message provided."
	if len(parts) > 0 {
		message = strings.Join(parts, " ")
	}
	b.discord.ChannelMessageSend(feedChannel, fmt.Sprintf(modRequest, m.Author.ID, m.ChannelID, url, message))
	b.sendQuickPM(m.Author.ID, fmt.Sprintf(requestRecv, m.Author.ID))
	b.discord.ChannelMessageDelete(m.ChannelID, m.ID)
}
