package mtabot

import (
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) cmdTopic(_ string, _ *discordgo.Session, m *discordgo.Message, _ []string) {
	c, err := b.Channel(m.GuildID, m.ChannelID)
	if err != nil {
		log.Println("!topic error - could not get channel " + m.ChannelID + " in guild " + m.GuildID)
		return
	}

	err = b.discord.ChannelMessageDelete(m.ChannelID, m.ID)
	if err != nil {
		log.Printf("!topic error - could not delete message %s in channel %s in guild %s\n", m.ID, m.ChannelID, m.GuildID)
	}

	_, err = b.discord.ChannelMessageSend(m.ChannelID, fmt.Sprintf("**Topic (requested by <@%s>)**\n%s", m.Author.ID, c.Topic))
	if err != nil {
		log.Println("!topic error - could not send to channel " + m.ChannelID + " in guild " + m.GuildID)
		return
	}
}
