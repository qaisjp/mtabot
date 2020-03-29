// build me using `go build -buildmode=plugin`
package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/multitheftauto/mtabot"
)

func init() {
	fmt.Println("Security module has been initialised")
}

type bot struct{ *mtabot.Bot }

func (b *bot) checkserial(cmd string, s *discordgo.Session, m *discordgo.Message, parts []string) {
	if ok, err := b.IsPrivateChannel(m.GuildID, m.ChannelID); err != nil || !ok {
		return
	}

	if !b.IsUserModerator(m.GuildID, m.Author.ID) {
		return
	}

	if os.Getenv("MTABOT_BASIC_AUTH") == "" {
		fmt.Println("MTABOT_BASIC_AUTH is missing")
		return
	}

	if len(parts) == 0 {
		fmt.Println("provide multiple serials pls")
		return
	}

	fmt.Println("loading")
	data, err := getBanData()
	if err != nil {
		fmt.Println("some error happened")
		return
	}
	fmt.Println("loaded")

	var embeds []*discordgo.MessageEmbed
	for _, serial := range parts {
		serial = strings.ToUpper(serial)
		bans := data.serialbans[serial]
		for _, ban := range bans {
			embeds = append(embeds, ban.toEmbed())
		}

		if len(bans) == 0 {
			embeds = append(embeds, &discordgo.MessageEmbed{
				Title:       serial,
				Description: "This serial has no associated bans.",
			})
		}
	}

	footer := &discordgo.MessageEmbedFooter{
		IconURL: m.Author.AvatarURL(""),
		Text:    "Information requested by " + m.Author.String(),
	}
	nowTimestamp := time.Now().Format(time.RFC3339)

	for i, embed := range embeds {
		if len(embeds) > 1 {
			embed.Author = &discordgo.MessageEmbedAuthor{Name: fmt.Sprintf("%02d of %02d", i+1, len(embeds))}
		}
		embed.Footer = footer
		embed.Timestamp = nowTimestamp

		_, err := s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{Embed: embed})
		if err != nil {
			fmt.Println("Message not sent", err.Error())
		}
	}
}

func Load(b *mtabot.Bot) {
	bot := &bot{b}
	b.AddCommand(bot.checkserial, "csdev")
}

// curl '' -H 'Connection: keep-alive' -H 'Pragma: no-cache' -H 'Cache-Control: no-cache' -H 'Authorization: Basic ***REMOVED***'
