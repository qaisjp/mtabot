package mtabot

import "github.com/bwmarrin/discordgo"

// IsModerator tests to see if that user is an "approved" role
func (b *bot) IsModerator(m *discordgo.Member) bool {
	for _, role := range m.Roles {
		if strSliceContains(modRoles, role) {
			return true
		}
	}
	return false
}

func (b *bot) IsUserModerator(guild string, user string) bool {
	m, err := b.discord.State.Member(guild, user)
	if err != nil || m == nil {
		return false
	}
	return b.IsModerator(m)
}

// IsAdmin tests to see if that user is an "approved" role
func (b *bot) IsAdmin(m *discordgo.Member) bool {
	for _, role := range m.Roles {
		if strSliceContains(adminRoles, role) {
			return true
		}
	}
	return false
}

func (b *bot) IsUserAdmin(guild string, user string) bool {
	m, err := b.discord.State.Member(guild, user)
	if err != nil || m == nil {
		return false
	}
	return b.IsAdmin(m)
}
