package main

import (
	"strings"

	"github.com/bwmarrin/discordgo"
)

func (b *bot) mee6inform(m *discordgo.Message, parts []string, msgIdx int) {
	if msgIdx >= len(parts) {
		return
	}

	source, err := b.discord.State.Member(m.GuildID, m.Author.ID)
	if err != nil {
		b.discord.ChannelMessageSend(m.ChannelID, "ERROR: Could not get source user guild member: "+err.Error())
		return
	}

	targetUser := parts[1]
	if !userRegexp.MatchString(targetUser) {
		return
	}

	targetUID := userRegexp.FindStringSubmatch(targetUser)[1]

	target, err := b.discord.State.Member(m.GuildID, targetUID)
	if err != nil {
		target, err = b.discord.GuildMember(m.GuildID, m.Author.ID)
		if err != nil {
			b.discord.ChannelMessageSend(m.ChannelID, "ERROR: Could not get target user guild member: "+err.Error())
			return
		}
	}

	if !b.canAction(source, target) {
		return
	}

	// Transform "!kick" to "kicked", "!ban" to "banned"
	cmd := parts[0][1:]
	if cmd != "kick" {
		cmd += "n"
	}
	cmd += "ed"

	msg := "You have been " + cmd

	if cmd == "tempbanned" {
		if len(parts) < 2 {
			return
		}
		msg += " for " + parts[2]
	}
	msg += " from Multi Theft Auto. Reason: " + strings.Join(parts[msgIdx:], " ")

	if err := b.sendQuickPM(targetUID, msg); err != nil {
		b.discord.ChannelMessageSend(m.ChannelID, "ERROR: "+err.Error())
	}
}
