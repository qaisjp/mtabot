package mtabot

import (
	"fmt"
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
		if strings.HasSuffix(a.Filename, ".dll") {
			heuristics = append(heuristics, "has suffix `.dll`")
		}

		if filenameValue != "" {
			filenameValue += ", "
		}
		filenameValue += a.Filename
	}

	if filenameValue == "" {
		filenameValue = "(none)"
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

	link := fmt.Sprintf("[Click here to read the message](%s)\n", composeMessageURL(m.Message))

	_, _ = s.ChannelMessageSendEmbed(feedChannel, &discordgo.MessageEmbed{
		Author: &discordgo.MessageEmbedAuthor{
			Name:    m.Author.String() + " (" + m.Author.ID + ")",
			IconURL: m.Author.AvatarURL(""),
		},
		Timestamp:   string(m.Timestamp),
		Color:       0xffa500,
		Title:       "A potentially malicious message has been sent",
		Description: link + heuristicText + ": " + strings.Join(heuristics, ", "),
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
