package mtabot

import (
	"log"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) cmdTopic(_ string, _ *discordgo.Session, m *discordgo.Message, _ []string) {
	c, err := b.Channel(m.GuildID, m.ChannelID)
	if err != nil {
		log.Println("!topic error - could not get channel " + m.ChannelID + " in guild " + m.GuildID)
		return
	}

	_, err = b.discord.ChannelMessageSend(m.ChannelID, "**Topic**\n"+c.Topic)
	if err != nil {
		log.Println("!topic error - could not send to channel " + m.ChannelID + " in guild " + m.GuildID)
		return
	}
}
