package mtabot

import (
	"strings"

	"github.com/bwmarrin/discordgo"
)

func (b *bot) checkMessageAttachments(s *discordgo.Session, m *discordgo.MessageCreate) {
	var heuristics []string
	filenameValue := ""
	for _, a := range m.Attachments {
		if strings.HasSuffix(a.Filename, ".exe") {
			heuristics = append(heuristics, "has suffix `.exe`")
		}

		if filenameValue != "" {
			filenameValue += ", "
		}
		filenameValue += a.Filename
	}

	heuristicText := "Heuristic"
	if length := len(heuristics); length == 0 {
		return
	} else if length > 1 {
		heuristicText += "s"
	}

	filenameText := "Filename"
	if len(m.Attachments) > 1 {
		filenameText += "s"
	}

	_, _ = s.ChannelMessageSendEmbed(feedChannel, &discordgo.MessageEmbed{
		URL: composeMessageURL(m.Message),
		Author: &discordgo.MessageEmbedAuthor{
			Name:    m.Author.String(),
			IconURL: m.Author.AvatarURL(""),
		},
		Timestamp:   string(m.Timestamp),
		Color:       0xffa500,
		Title:       "A potentially malicious message has been sent",
		Description: heuristicText + ": " + strings.Join(heuristics, ", "),
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   filenameText,
				Value:  filenameValue,
				Inline: true,
			},
			{
				Name:   "Channel",
				Value:  "<#" + m.ChannelID + ">",
				Inline: true,
			},
		},
	})
}
