package mtabot

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

var errNoUserPchat = errors.New("could not find user pchat")

func (b *bot) findUserPchat(user string) (*discordgo.Channel, error) {
	chans, err := b.discord.GuildChannels(guild)
	if err != nil {
		return nil, errors.Wrap(err, "could not get guild channels")
	}

	for _, c := range chans {
		if c.ParentID == pchatCategory {
			info, err := pchatExtractTopicInfo(c.Topic)
			if err == nil && info.UserID == user {
				return c, nil
			}
		}
	}

	return nil, errNoUserPchat
}

func pchatExtractTopicInfo(topic string) (info pchatInfo, err error) {
	list := strings.Split(topic, pChatInfoSeparator)
	if len(list) != 2 {

		return info, errors.New("topic is malformed")
	}

	err = json.Unmarshal([]byte(list[1]), &info)
	if err != nil {
		return info, errors.New("topic is malformed")
	}

	return
}

func (b *bot) cmdPchat(cmd string, s *discordgo.Session, m *discordgo.Message, parts []string) {
	if len(parts) == 0 {
		ch, err := b.findUserPchat(m.Author.ID)
		if err != nil && err != errNoUserPchat {
			fmt.Printf("[ERROR] failed to check if pchat exists, self-requested by user %s: %s", m.Author.ID, err.Error())
			return
		} else if ch != nil {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("<@%s>, you can visit your existing private chat here: <#%s>.", m.Author.ID, ch.ID))
			return
		}

		ch, err = createPchatChannel(s, m.Author, true)
		if err != nil {
			fmt.Printf("[ERROR] failed to create pchat requested by user %s: %s", m.Author.ID, err.Error())
			return
		}

		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("<@%s>, you have been invited to <#%s>.", m.Author.ID, ch.ID))
		return
	}

	if parts[0] == "start" || parts[0] == "stop" || parts[0] == "archive" {
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
		info, err := pchatExtractTopicInfo(channel.Topic)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "ERROR: "+err.Error())
			return
		}

		// Ensure the dude exists
		_, err = b.Member(m.GuildID, info.UserID)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "ERROR: could not retrieve target user: "+err.Error())
			return
		}

		if parts[0] == "start" {
			err = s.ChannelPermissionSet(m.ChannelID, info.UserID, "member", discordgo.PermissionReadMessages, 0)
		} else if parts[0] == "stop" || parts[0] == "archive" {
			err = s.ChannelPermissionDelete(m.ChannelID, info.UserID)
		}

		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "ERROR: Could not set target user channel permissions: "+err.Error())
			return
		}

		if parts[0] == "archive" {
			if _, err := s.ChannelEditComplex(m.ChannelID, &discordgo.ChannelEdit{
				ParentID: archiveCategory,
			}); err != nil {
				s.ChannelMessageSend(m.ChannelID, "ERROR: Could not set parent ID: "+err.Error())
				return
			}

		}

		b.okHand(m)

		return
	}

	targetUser := parts[0]
	if !userRegexp.MatchString(targetUser) {
		return
	}

	if !b.IsUserModerator(m.GuildID, m.Author.ID) {
		return
	}

	targetUID := userRegexp.FindStringSubmatch(targetUser)[1]

	target, err := b.Member(m.GuildID, targetUID)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "ERROR: Could not get target user guild member: "+err.Error())
		return
	}

	ch, err := createPchatChannel(s, target.User, false)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "ERROR: "+err.Error())
		return
	}

	// Send quick PM
	if err := b.sendQuickPM(target.User.ID, "You have been invited to <#"+ch.ID+">."); err != nil {
		s.ChannelMessageSend(m.ChannelID, "ERROR: "+err.Error())
	}
}

func createPchatChannel(s *discordgo.Session, user *discordgo.User, selfRequested bool) (*discordgo.Channel, error) {
	info := pchatInfo{user.ID}
	infoBytes, err := json.Marshal(info)
	if err != nil {
		return nil, errors.Wrap(err, "fatal error - could not marshal json")
	}

	channel, err := s.GuildChannelCreateComplex(guild, discordgo.GuildChannelCreateData{
		Name:     user.Username + "-" + user.Discriminator,
		Type:     discordgo.ChannelTypeGuildText,
		Topic:    pChatInstructions + "\n\n\n" + pChatInfoSeparator + string(infoBytes),
		ParentID: pchatCategory,
		NSFW:     false,
	})
	if err != nil {
		return nil, errors.Wrap(err, "could not create pchat channel")
	}

	s.ChannelMessageSend(channel.ID, pChatInstructions)
	if selfRequested {
		s.ChannelMessageSend(channel.ID, fmt.Sprintf("@here, this room was self-requested by <@%s>.", user.ID))
	}

	err = s.ChannelPermissionSet(channel.ID, info.UserID, "member", discordgo.PermissionReadMessages, 0)
	if err != nil {
		s.ChannelMessageSend(channel.ID, "ERROR: could not give user read permission to channel: "+err.Error())
	}

	return channel, nil
}
