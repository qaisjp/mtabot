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
	"github.com/pkg/errors"
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
const feedChannel = "486298246138953749"
const privateChannelGroup = "360863453348626452"

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

var mee6inform = map[string]int{
	"!ban":     2,
	"!tempban": 3,
	"!kick":    2,
}

func (b *bot) onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.GuildID == "" {
		b.pchatPM(s, m.Message)
		return
	}

	// Limit to MTA guild only
	if m.GuildID != guild {
		return
	}

	fmt.Printf("[%s] in %s by %s\t%s\n", m.Timestamp, m.ChannelID, m.Author.ID, m.Content)

	// Ignore all messages created by the bot itself
	// This isn't required in this specific example but it's a good practice.
	if m.Author.ID == s.State.User.ID {
		return
	}

	channel, err := s.Channel(m.ChannelID)
	if err != nil {
		fmt.Println("[error] unknown channel", m.ChannelID)
		return
	}

	if channel.ParentID != privateChannelGroup {
		b.checkMessageAttachments(s, m)
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
	} else if parts[0] == "!mod" || parts[0] == "!mods" {
		b.requestMod(m.Message, parts[1:])
		return
	} else if idx, ok := mee6inform[parts[0]]; ok {
		b.mee6inform(m.Message, parts, idx)
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

func (b *bot) sendQuickPM(user string, message string) (err error) {
	ch, err := b.discord.UserChannelCreate(user)
	if err != nil {
		return errors.Wrap(err, "could not create PM channel")
	}

	_, err = b.discord.ChannelMessageSend(ch.ID, message)
	if err != nil {
		return errors.Wrap(err, "could not send PM message")
	}

	_, err = b.discord.ChannelDelete(ch.ID)
	if err != nil {
		return errors.Wrap(err, "could not delete PM channel")
	}
	return
}
