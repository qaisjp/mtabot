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

		ch, err = createPchatChannel(s, m.Author, m.Author)
		if err != nil {
			fmt.Printf("[ERROR] failed to create pchat requested by user %s: %s", m.Author.ID, err.Error())
			return
		}

		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("<@%s>, you have been invited to <#%s>.", m.Author.ID, ch.ID))
		return
	}

	shouldStart := parts[0] == "start" || parts[0] == "open"
	shouldStop := parts[0] == "stop" || parts[0] == "close"
	shouldArchive := parts[0] == "archive"

	if shouldStart || shouldStop || shouldArchive {
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
		if err == nil {
			if shouldStart {
				err = s.ChannelPermissionSet(m.ChannelID, info.UserID, "member", discordgo.PermissionReadMessages, 0)
			} else if shouldStop || shouldArchive {
				err = s.ChannelPermissionDelete(m.ChannelID, info.UserID)
			}

			if err != nil {
				s.ChannelMessageSend(m.ChannelID, "ERROR: Could not set target user channel permissions: "+err.Error())
				return
			}
		} else if shouldStart {
			s.ChannelMessageSend(m.ChannelID, "Btw that user has left the server... I think. "+err.Error())
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

	ch, err := createPchatChannel(s, target.User, m.Author)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "ERROR: "+err.Error())
		return
	}

	// Send quick PM
	if err := b.sendQuickPM(target.User.ID, "You have been invited to <#"+ch.ID+">."); err != nil {
		s.ChannelMessageSend(m.ChannelID, "ERROR: "+err.Error())
	}
}

func createPchatChannel(s *discordgo.Session, user *discordgo.User, requestedBy *discordgo.User) (*discordgo.Channel, error) {
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

	err = s.ChannelPermissionSet(channel.ID, info.UserID, "member", discordgo.PermissionReadMessages, 0)
	if err != nil {
		s.ChannelMessageSend(channel.ID, "ERROR: could not give user read permission to channel: "+err.Error())
	}

	prefix := fmt.Sprintf("Hello <@%s>", user.ID)
	if requestedBy.ID != user.ID {
		prefix += fmt.Sprintf(", you've been invited by <@%s>", requestedBy.ID)
	}

	embed := discordgo.MessageEmbed{
		Title: "IMPORTANT INFORMATION",
		Description: prefix + `!

If your question is related to:
1. MTA not working properly or other computer problems, use <#278521065435824128>.
2. scripting, use <#278520948347502592>, <#667822335125880835> or <#667822318743060490>.
3. a PERMANENT global game ban, you can [appeal bans on the forum](https://forum.mtasa.com/forum/180-ban-appeals/).
4. a TEMPORARY global game ban, you CANNOT appeal the ban. Type ` + "`!pchat stop`" + ` if you have been tempbanned.
5. a complaint about server list abuse, log in on forum.mtasa.com and then [make a post here](https://forum.mtasa.com/forum/188-server-list-abuse-help/). You need to log in first.
6. moving your server rank - we no longer provide the service of moving your server rank when your IP address changes

If you have any other question, please state your question and wait patiently for a response. Do not @mention staff.`,
	}

	s.ChannelMessageSendEmbed(channel.ID, &embed)

	return channel, nil
}
