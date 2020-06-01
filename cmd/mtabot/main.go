package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"plugin"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/multitheftauto/mtabot"
)

func main() {
	tokenBytes, err := ioutil.ReadFile("token.txt")
	if err != nil {
		panic(err)
	}

	secFn := loadsec()

	discord, err := discordgo.New("Bot " + strings.TrimSpace(string(tokenBytes)))
	if err != nil {
		panic(err)
	}
	discord.StateEnabled = true

	bot := mtabot.NewBot(discord)

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

	if secFn != nil {
		secFn(bot)
		fmt.Println("Loaded security plugin")
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

func loadsec() func(*mtabot.Bot) {
	secpath := os.Getenv("MTABOT_SECURITY_PLUGIN")
	if secpath == "" {
		fmt.Println("No security plugin")
		return nil
	}

	var err error
	secplugin, err := plugin.Open(secpath)
	if err != nil {
		panic(err.Error())
	}

	fnLookup, err := secplugin.Lookup("Load")
	if err != nil {
		panic(err.Error())
	}

	fnP, ok := fnLookup.(func(*mtabot.Bot))
	if !ok {
		panic(fmt.Sprintf("Load function in security plugin of incorrect type, has %T wanted %T", fnLookup, fnP))
	}

	return fnP
}
