package mtabot

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var karmaRegexp = regexp.MustCompile(`^<@!?(\d+)> ?(\+\+|--)(?: (.*))?$`)

func (b *bot) cmdKarma(cmd string, s *discordgo.Session, m *discordgo.Message, parts []string) {
	target := m.Author.ID
	if len(parts) > 0 {
		if !userRegexp.MatchString(parts[0]) {
			return
		}
		target = userRegexp.FindStringSubmatch(parts[0])[1]
	}

	if len(parts) <= 1 {
		b.karmaGet(m, target)
		return
	}

	// Ignore command if more than 2
	if len(parts) > 2 {
		return
	}

	// Ignore command if author is not an admin
	if !b.IsUserAdmin(m.GuildID, m.Author.ID) {
		return
	}

	numStr := parts[1]
	relative := strings.HasPrefix(numStr, "r")
	if relative {
		numStr = numStr[1:]
	}

	new, err := strconv.Atoi(numStr)
	if err != nil {
		return
	}

	if relative {
		new += b.Karma.Get(target)
	}

	_, err = b.Karma.Set(target, new)
	if err != nil {
		b.discord.ChannelMessageSend(m.ChannelID, "ERROR: Could not update karma: "+err.Error())
		return
	}

	b.karmaGet(m, target)
}

func (b *bot) karmaGet(m *discordgo.Message, uid string) {
	karma := b.Karma.Get(uid)
	member, err := b.Member(m.GuildID, uid)
	if err != nil {
		b.discord.ChannelMessageSend(m.ChannelID, "ERROR: Could not get target user name: "+err.Error())
		return
	}

	b.discord.ChannelMessageSend(m.ChannelID, fmt.Sprintf("%s has %d karma", MemberName(member), karma))
}

func (b *bot) karmaAction(m *discordgo.Message, uid string, positive bool, reason string) {
	// If performing action on self, make it negative
	if uid == m.Author.ID {
		positive = false
	}

	add := 1
	if positive == false {
		add = -1
	}

	new, err := b.Karma.Set(uid, b.Karma.Get(uid)+add)
	if err != nil {
		b.discord.ChannelMessageSend(m.ChannelID, "ERROR: Could not update karma: "+err.Error())
		return
	}

	member, err := b.Member(m.GuildID, uid)
	if err != nil {
		b.discord.ChannelMessageSend(m.ChannelID, "ERROR: Could not get target user name: "+err.Error())
		return
	}

	// Strip @everyone
	reason = stripEveryone(m.GuildID, reason)

	b.discord.ChannelMessageSend(m.ChannelID, fmt.Sprintf("%s now has %d karma %s", MemberName(member), new, reason))
}

type karmaBox struct {
	filename string
	m        map[string]int
}

func (k *karmaBox) Get(user string) int {
	return k.m[user]
}

func (k *karmaBox) Set(user string, new int) (newKarma int, err error) {
	k.m[user] = new
	if err := k.Save(); err != nil {
		return 0, err
	}
	return k.m[user], nil
}

func (k *karmaBox) Save() error {
	b, err := json.Marshal(k.m)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(k.filename, b, 0644)
}

func NewKarmaBox(filename string) (*karmaBox, error) {
	box := karmaBox{filename: filename}

	b := []byte("{}")
	if _, err := os.Stat(filename); !os.IsNotExist(err) {
		b, err = ioutil.ReadFile(filename)
		if err != nil {
			return nil, err
		}
	}

	if err := json.Unmarshal(b, &box.m); err != nil {
		return nil, err
	}

	return &box, nil
}
