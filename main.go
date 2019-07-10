package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

// CONFIG
const guild = "278474088903606273"
const muteRole = "560113810577555476"
const pchatCategory = "584767048035467304"
const archiveCategory = "584765668470161408"

var modRoles = []string{
	"278474612755398667", // mta team
	"278932343828381696", // administrators
	"283590599808909332", // moderators
}

const modLogChannel = "303958138489667584"

const pChatInfoSeparator = "DO NOT MODIFY FROM THIS POINT ONWARDS:"
const pChatInstructions = "- Use `!pchat start` to invite the user to this channel." + `
` + "- Use `!pchat stop` to remove the user." + `
` + "- Ask an admin to archive or unarchive."

// END_CONFIG

const okHandEmoji = "👌"

var userRegexp = regexp.MustCompile(`<@!?(\d+)>`)
var luaRegexp = regexp.MustCompile(`(^|\W)LUA($|\W)`)

const itsLuaMessage = `It's Lua, not LUA. https://www.lua.org/about.html
` + "```" + `"Lua" (pronounced LOO-ah) means "Moon" in Portuguese. As such, it is neither an acronym nor an abbreviation, ` +
	`but a noun. More specifically, "Lua" is a name, the name of the Earth's moon and the name of the language. ` +
	`Like most names, it should be written in lower case with an initial capital, that is, "Lua". ` +
	`Please do not write it as "LUA", which is both ugly and confusing, because then it becomes an acronym ` +
	`with different meanings for different people. So, please, write "Lua" right!
` + "```"

type bot struct {
	discord *discordgo.Session
	karma   *karmaBox
}

func main() {
	tokenBytes, err := ioutil.ReadFile("token.txt")
	if err != nil {
		panic(err)
	}

	discord, err := discordgo.New("Bot " + strings.TrimSpace(string(tokenBytes)))
	if err != nil {
		panic(err)
	}
	discord.StateEnabled = true

	karma, err := newKarmaBox("karma.json")
	if err != nil {
		panic(err)
	}

	bot := bot{discord, karma}

	discord.AddHandler(bot.onMessageCreate)

	// Open a websocket connection to Discord and begin listening.
	err = discord.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}

	resp, err := discord.GatewayBot()
	if err != nil {
		panic(err)
	}

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	fmt.Printf("Gateway: %s\nRecommended number of shards: %d\n", resp.URL, resp.Shards)
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Cleanly close down the Discord session.
	discord.Close()
}

func (b *bot) onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Limit to MTA guild only
	if m.GuildID != guild {
		return
	}

	fmt.Println(m.Content)

	// Ignore all messages created by the bot itself
	// This isn't required in this specific example but it's a good practice.
	if m.Author.ID == s.State.User.ID {
		return
	}

	if karmaRegexp.MatchString(m.Content) {
		parts := karmaRegexp.FindStringSubmatch(m.Content)

		uid := parts[1]
		positive := parts[2] == "++"
		reason := ""
		if len(parts) == 4 {
			reason = parts[3]
		}
		b.karmaAction(m.Message, uid, positive, reason)
	}

	parts := strings.Split(m.Content, " ")

	if parts[0] == "!cmute" || parts[0] == "!cunmute" {
		shouldMute := parts[0] == "!cmute"
		b.muteAction(s, m.Message, parts, shouldMute)
		return
	} else if parts[0] == "!pchat" {
		if len(parts) == 1 {
			return
		}
		b.privateChatAction(s, m.Message, parts[1:])
		return
	} else if parts[0] == "!karma" {
		target := m.Author.ID
		if len(parts) == 2 {
			if !userRegexp.MatchString(parts[1]) {
				return
			}
			target = userRegexp.FindStringSubmatch(parts[1])[1]
		}
		b.karmaGet(m.Message, target)
		return
	}

	// If the message contains "LUA" reply with a "Lua not LUA" message
	if luaRegexp.Match([]byte(m.Content)) {
		s.ChannelMessageSend(m.ChannelID, itsLuaMessage)
	}
}

// canAction tests if the source user can perform that action against that user
func (b *bot) canAction(source, target *discordgo.Member) bool {
	if b.isModerator(target) {
		return false
	}
	return b.isModerator(source)
}

// isModerator tests to see if that user is an "approved" role
func (b *bot) isModerator(m *discordgo.Member) bool {
	for _, role := range m.Roles {
		if isModRole(role) {
			return true
		}
	}
	return false
}

// okHand sends an :ok_hand: emoji to the message
func (b *bot) okHand(m *discordgo.Message) {
	err := b.discord.MessageReactionAdd(m.ChannelID, m.ID, okHandEmoji)
	if err != nil {
		fmt.Printf("WARNING: could not add message reaction: %s\n", err)
	}
}

type pchatInfo struct {
	UserID string
}

func (b *bot) privateChatAction(s *discordgo.Session, m *discordgo.Message, parts []string) {
	fmt.Println(strings.Join(parts, ","))
	if parts[0] == "start" || parts[0] == "stop" || parts[0] == "archive" {
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
			aPerms, err := s.State.UserChannelPermissions(info.UserID, m.ChannelID)
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, "ERROR: Could not get target user channel permissions: "+err.Error())
				return
			}

			dPerms := discordgo.PermissionReadMessageHistory
			aPerms &= ^discordgo.PermissionReadMessageHistory
			aPerms |= discordgo.PermissionReadMessages | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks | discordgo.PermissionAttachFiles

			err = s.ChannelPermissionSet(m.ChannelID, info.UserID, "member", aPerms, dPerms)
		} else if parts[0] == "stop" || parts[0] == "archive" {
			err = s.ChannelPermissionDelete(m.ChannelID, info.UserID)
		}

		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "ERROR: Could not set target user channel permissions: "+err.Error())
			return
		}

		if parts[0] == "archive" {
			// Get position of archive channel
			archiveCategory, err := b.Channel(m.GuildID, pchatCategory)
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, "ERROR: Could not get the archive category channel: "+err.Error())
				return
			}

			channel.Position = archiveCategory.Position + 1
			err = s.GuildChannelsReorder(m.GuildID, []*discordgo.Channel{channel})
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, "ERROR: Could not move this channel: "+err.Error())
				return
			}
			s.ChannelMessageSend(m.ChannelID, "Done!")
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

	info := pchatInfo{targetUID}
	infoBytes, err := json.Marshal(info)
	if err != nil {
		panic(err)
	}

	channel, err := s.GuildChannelCreateComplex(m.GuildID, discordgo.GuildChannelCreateData{
		Name:     target.User.Username + "-" + target.User.Discriminator,
		Type:     discordgo.ChannelTypeGuildText,
		Topic:    pChatInstructions + "\n\n\n" + pChatInfoSeparator + string(infoBytes),
		ParentID: pchatCategory,
		NSFW:     false,
	})
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "ERROR: Could not create pchat channel: "+err.Error())
		return
	}

	s.ChannelMessageSend(channel.ID, pChatInstructions)
}

func (b *bot) Member(guildID, userID string) (*discordgo.Member, error) {
	m, err := b.discord.State.Member(guildID, userID)
	if err == nil {
		return m, nil
	}

	m, err = b.discord.GuildMember(guildID, userID)
	if err == nil {
		return m, nil
	}

	return nil, err
}

func (b *bot) Channel(guildID, channelID string) (*discordgo.Channel, error) {
	m, err := b.discord.State.GuildChannel(guildID, channelID)
	if err == nil {
		return m, nil
	}

	m, err = b.discord.Channel(channelID)
	if err == nil {
		return m, nil
	}

	return nil, err
}

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
	} else if !b.isModerator(source) {
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

func isModRole(role string) bool {
	for _, s := range modRoles {
		if s == role {
			return true
		}
	}
	return false
}

func composeMessageURL(m *discordgo.Message) string {
	return "https://discordapp.com/channels/" + m.GuildID + "/" + m.ChannelID + "/" + m.ID
}

func channelsRemove(slice []*discordgo.Channel, cid string) []*discordgo.Channel {
	var i int
	for index, v := range slice {
		if v.ID == cid {
			i = index
			break
		}
	}

	return append(slice[:i], slice[i+1:]...)
}

func channelsInsertAfter(s []*discordgo.Channel, after string, value *discordgo.Channel) []*discordgo.Channel {
	var i int
	for index, v := range s {
		if v.ID == after {
			i = index + 1
			break
		}
	}

	if len(s) == i {
		return append(s, value)
	}

	s = append(s, nil)
	copy(s[i+1:], s[i:])
	s[i] = value
	return s
}

func printChannels(chans []*discordgo.Channel) {
	fmt.Println("channels: {")
	for _, c := range chans {
		fmt.Printf("\t%s @ %d (%s)\n", c.Name, c.Position, c.ID)
	}
	fmt.Println("}")
}

func MemberName(member *discordgo.Member) string {
	if member.Nick != "" {
		return member.Nick
	}
	return member.User.Username
}

func everyone(guildID string) string {
	return "<@&" + guildID + ">"
}

func stripEveryone(guildID string, message string) string {
	message = strings.ReplaceAll(message, everyone(guildID), "")
	message = strings.ReplaceAll(message, "@everyone", "")
	message = strings.ReplaceAll(message, "@here", "")
	return message
}
