package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/pkg/errors"
)

const pChatInfoSeparator = "DO NOT MODIFY FROM THIS POINT ONWARDS:"
const pChatInstructions = "- Use `!pchat start` to re-invite the user to this channel." + `
` + "- Use `!pchat stop` to remove the user." + `
` + "- The user has automatically been invited to this channel."

type pchatInfo struct {
	UserID string
}

func (b *bot) pchatPM(s *discordgo.Session, m *discordgo.Message) {
	// First ensure member is in the MTA guild
	_, err := b.Member(guild, m.Author.ID)
	if err != nil {
		fmt.Printf("pchatPM error: %s\n", err.Error())
		return
	}
}

func (b *bot) privateChatAction(s *discordgo.Session, m *discordgo.Message, parts []string) {
	if len(parts) == 0 {
		if err := createPchatChannel(s, m.Author, true); err != nil {
			fmt.Printf("[ERROR] failed to create pchat requested by user %s: %s", m.Author.ID, err.Error())
		}
		return
	}

	fmt.Println(strings.Join(parts, ","))
	if parts[0] == "start" || parts[0] == "stop" {
		var info pchatInfo

		channel, err := s.State.Channel(m.ChannelID)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "ERROR: Could not load channel info: "+err.Error())
			return
		}

		// Check parent
		if channel.ParentID != pchatCategory {
			s.ChannelMessageSend(m.ChannelID, "DENIED: This channel is not inside the p-chat category")
			return
		}

		// Extract info from topic
		{
			list := strings.Split(channel.Topic, pChatInfoSeparator)
			if len(list) != 2 {
				s.ChannelMessageSend(m.ChannelID, "ERROR: Topic is malformed")
				return
			}

			err := json.Unmarshal([]byte(list[1]), &info)
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, "ERROR: Topic is malformed")
				return
			}
		}

		// Ensure the dude exists
		_, err = b.Member(m.GuildID, info.UserID)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "ERROR: could not retrieve target user: "+err.Error())
			return
		}

		if parts[0] == "start" {
			err = s.ChannelPermissionSet(m.ChannelID, info.UserID, "member", discordgo.PermissionReadMessages, 0)
		} else if parts[0] == "stop" {
			err = s.ChannelPermissionDelete(m.ChannelID, info.UserID)
		}

		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "ERROR: Could not set target user channel permissions: "+err.Error())
			return
		}

		b.okHand(m)

		return
	}

	targetUser := parts[0]
	if !userRegexp.MatchString(targetUser) {
		return
	}

	targetUID := userRegexp.FindStringSubmatch(targetUser)[1]

	target, err := b.Member(m.GuildID, targetUID)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "ERROR: Could not get target user guild member: "+err.Error())
		return
	}

	if err := createPchatChannel(s, target.User, false); err != nil {
		s.ChannelMessageSend(m.ChannelID, "ERROR: "+err.Error())
	}
}

func createPchatChannel(s *discordgo.Session, user *discordgo.User, selfRequested bool) error {
	info := pchatInfo{user.ID}
	infoBytes, err := json.Marshal(info)
	if err != nil {
		panic(err)
	}

	channel, err := s.GuildChannelCreateComplex(guild, discordgo.GuildChannelCreateData{
		Name:     user.Username + "-" + user.Discriminator,
		Type:     discordgo.ChannelTypeGuildText,
		Topic:    pChatInstructions + "\n\n\n" + pChatInfoSeparator + string(infoBytes),
		ParentID: pchatCategory,
		NSFW:     false,
	})
	if err != nil {
		return errors.Wrap(err, "could not create pchat channel")
	}

	s.ChannelMessageSend(channel.ID, pChatInstructions)
	s.ChannelMessageSend(channel.ID, fmt.Sprintf("@here, this room was self-requested by <@%s>.", user.ID))

	err = s.ChannelPermissionSet(channel.ID, info.UserID, "member", discordgo.PermissionReadMessages, 0)
	if err != nil {
		s.ChannelMessageSend(channel.ID, "ERROR: could not give user read permission to channel: "+err.Error())
	}
	return nil
}
