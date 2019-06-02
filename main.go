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

	bot := bot{discord}

	discord.AddHandler(bot.onMessageCreate)

	// Open a websocket connection to Discord and begin listening.
	err = discord.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Cleanly close down the Discord session.
	discord.Close()
}

func (b *bot) onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore all messages created by the bot itself
	// This isn't required in this specific example but it's a good practice.
	if m.Author.ID == s.State.User.ID {
		return
	}

	// If the message contains "LUA" reply with a "Lua not LUA" message
	if luaRegexp.Match([]byte(m.Content)) {
		s.ChannelMessageSend(m.ChannelID, itsLuaMessage)
	}
}
