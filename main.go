package mtabot

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/pkg/errors"
)

// CONFIG
const guild = "278474088903606273"
const muteRole = "560113810577555476"
const pchatCategory = "584767048035467304"
const archiveCategory = "584765668470161408"

var modRoles = []string{
	"278932343828381696", // administrators
	"283590599808909332", // staff
	"584757327047950378", // retired mod
	"584817557492465674", // retired mta team
}

var adminRoles = []string{
	"278932343828381696", // administrators
	"584817557492465674", // retired mta team
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

type Bot struct {
	discord *discordgo.Session
	Karma   *karmaBox

	commands map[string]GenericCommand
}

type bot = Bot

func NewBot(discord *discordgo.Session) *Bot {
	b := &Bot{
		discord:  discord,
		commands: make(map[string]GenericCommand),
	}
	discord.AddHandler(b.onMessageCreate)
	b.AddCommand(b.cmdTopic, "topic")
	b.AddCommand(b.cmdKarma, "karma")
	b.AddCommand(b.cmdPchat, "pchat")
	b.AddCommand(b.cmdMute, "cmute", "cunmute")
	b.AddCommand(b.cmdModPing, "mod", "mods")
	return b
}

var mee6inform = map[string]int{
	"!ban":     2,
	"!tempban": 3,
	"!kick":    2,
}

type GenericCommand func(cmd string, s *discordgo.Session, m *discordgo.Message, parts []string)

func (b *bot) AddCommand(fn GenericCommand, cmds ...string) bool {
	if fn == nil {
		panic("I've been passed a bloody nil func")
	}

	for _, cmd := range cmds {
		_, exists := b.commands[cmd]
		if exists {
			return false
		}

		b.commands[cmd] = fn
	}
	return true
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

	parts := strings.Fields(m.Content)
	if len(parts) == 0 {
		return
	}

	if strings.HasPrefix(parts[0], "!") {
		cmd := parts[0][1:]
		fn, ok := b.commands[cmd]
		if ok {
			fn(cmd, s, m.Message, parts[1:])
			return
		}
	}

	// If the message contains "LUA" reply with a "Lua not LUA" message
	if luaRegexp.Match([]byte(m.Content)) {
		s.ChannelMessageSend(m.ChannelID, itsLuaMessage)
	}
}

// canAction tests if the source user can perform that action against that user
func (b *bot) canAction(source, target *discordgo.Member) bool {
	if b.IsModerator(target) {
		return false
	}
	return b.IsModerator(source)
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
