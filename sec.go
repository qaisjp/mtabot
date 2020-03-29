// build me using `go build -buildmode=plugin`
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/multitheftauto/mtabot"
)

const emojiLoading = "⌛"

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

	if err := s.MessageReactionAdd(m.ChannelID, m.ID, emojiLoading); err != nil {
		fmt.Printf("failed to add emoji: %s\n", err.Error())
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
		var bans []*banitem
		var typ string
		var description string

		if len(serial) == 32 {
			typ = "serial"
			serial = strings.ToUpper(serial)
			bans = data.serialbans[serial]
		} else {
			typ = "repid"
			id, err := strconv.Atoi(serial)
			if err != nil {
				description = err.Error()
			} else if ban, ok := data.repids[id]; ok {
				bans = []*banitem{ban}
			}
		}

		if description == "" && len(bans) == 0 {
			description = "This " + typ + " has no associated bans."
		} else if description == "" {
			for _, ban := range bans {
				embeds = append(embeds, ban.toEmbed())
			}
		} else {
			embeds = append(embeds, &discordgo.MessageEmbed{
				Title:       serial,
				Description: description,
				Color:       0x777777,
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

	s.MessageReactionRemove(m.ChannelID, m.ID, emojiLoading, "@me")
}

func Load(b *mtabot.Bot) {
	bot := &bot{b}
	b.AddCommand(bot.checkserial, "csdev")
}

// curl '' -H 'Connection: keep-alive' -H 'Pragma: no-cache' -H 'Cache-Control: no-cache' -H 'Authorization: Basic ***REMOVED***'
