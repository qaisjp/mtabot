package main

import (
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

var modRoles = []string{
	"278474612755398667", // mta team
	"278932343828381696", // administrators
	"283590599808909332", // moderators
}

const modLogChannel = "278517063587463171"

// END_CONFIG

var luaRegexp = regexp.MustCompile(`(\W)?LUA(\W)?`)

const itsLuaMessage = `It's Lua, not LUA. https://www.lua.org/about.html
` + "```" + `"Lua" (pronounced LOO-ah) means "Moon" in Portuguese. As such, it is neither an acronym nor an abbreviation, ` +
	`but a noun. More specifically, "Lua" is a name, the name of the Earth's moon and the name of the language. ` +
	`Like most names, it should be written in lower case with an initial capital, that is, "Lua". ` +
	`Please do not write it as "LUA", which is both ugly and confusing, because then it becomes an acronym ` +
	`with different meanings for different people. So, please, write "Lua" right!
` + "```"

type bot struct {
	discord *discordgo.Session
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

	bot := bot{discord}

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

	parts := strings.Split(m.Content, " ")

	if parts[0] == "!cmute" || parts[0] == "!cunmute" {
		shouldMute := parts[0] == "!cmute"
		b.muteAction(s, m.Message, parts, shouldMute)
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

func (b *bot) muteAction(s *discordgo.Session, m *discordgo.Message, parts []string, shouldMute bool) {
	if len(parts) < 2 {
		return
	}

	targetUser := parts[1]
	if len(targetUser) <= 3 || targetUser[:2] != "<@" || targetUser[len(targetUser)-1] != '>' {
		return
	}

	targetUID := targetUser[2 : len(targetUser)-1]

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

	err = s.MessageReactionAdd(m.ChannelID, m.ID, "🆗")
	if err != nil {
		fmt.Printf("WARNING: could not add message reaction: %s\n", err)
	}

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
