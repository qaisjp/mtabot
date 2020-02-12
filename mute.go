package mtabot

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func (b *bot) muteAction(s *discordgo.Session, m *discordgo.Message, parts []string, shouldMute bool) {
	if len(parts) < 2 {
		return
	}

	targetUser := parts[1]
	if !userRegexp.MatchString(targetUser) {
		return
	}

	targetUID := userRegexp.FindStringSubmatch(targetUser)[1]

	reason := ""
	if len(parts) > 2 {
		reason = strings.Join(parts[2:], " ")
	}

	source, err := s.State.Member(m.GuildID, m.Author.ID)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "ERROR: Could not get source user guild member: "+err.Error())
		return
	}

	if targetUID == m.Author.ID {
		err := s.MessageReactionAdd(m.ChannelID, m.ID, ":suicide:356175332002496512")
		if err != nil {
			fmt.Printf("WARNING: could not add message reaction: %s\n", err)
		}
		return
	} else if !b.IsModerator(source) {
		fmt.Printf("Non elevated user <@%s> (%s#%s) attempted to use elevated command\n", m.Author.ID, m.Author.Username, m.Author.Discriminator)
		return
	}

	target, err := s.State.Member(m.GuildID, targetUID)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "ERROR: Could not get target user guild member: "+err.Error())
		return
	}

	if !b.canAction(source, target) {
		// if shouldMute {
		// 	s.ChannelMessageSend(m.ChannelID, ":x: You can't mute a moderator")
		// }
		// return
	}

	action := "unmuted"
	if shouldMute {
		action = "muted"
	}

	aPerms, err := s.State.UserChannelPermissions(targetUID, m.ChannelID)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "ERROR: Could not get target user channel permissions: "+err.Error())
		return
	}

	dPerms := discordgo.PermissionSendMessages
	aPerms &= ^dPerms

	if shouldMute {
		err = s.ChannelPermissionSet(m.ChannelID, targetUID, "member", aPerms, dPerms)
	} else {
		err = s.ChannelPermissionDelete(m.ChannelID, targetUID)
	}

	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "ERROR: Could not set target user channel permissions: "+err.Error())
		return
	}

	b.okHand(m)

	// Inform in modlog channel
	url := composeMessageURL(m)
	s.ChannelMessageSend(modLogChannel, m.Author.Username+` has `+action+` `+targetUser+` (`+targetUID+`) in <#`+m.ChannelID+`> for reason: `+"\n```"+reason+"\n```\nHere: "+url)

}
