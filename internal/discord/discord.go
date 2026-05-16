package discord

import (
	"dailysongbot/internal/config"
	"dailysongbot/internal/errorlog"
	"dailysongbot/internal/youtube"
	"fmt"
	"net/url"
	"sync"
	"sync/atomic"

	"github.com/bwmarrin/discordgo"
)

var (
	DiscordSession *discordgo.Session
	isConnected    atomic.Bool
	canSend        = sync.NewCond(&sync.Mutex{})
)

// set status on connection and reconnection
// enable sending messages
func HandleConnection() {
	err := setDiscordStatus("preparing next song")
	if err != nil {
		errorlog.Logger.Warn(fmt.Errorf("set client status: %w", err).Error())
	}
	isConnected.Store(true)

	canSend.Broadcast()
}

// on disconnection disable sending messages
func HandleDisconnect() {
	isConnected.Store(false)
}

// start discord session
func StartDiscordClient() error {
	var (
		err   error
		token string
	)

	token = fmt.Sprintf("Bot %s", config.EnvConfig.DiscordBotToken)
	DiscordSession, err = discordgo.New(token)
	if err != nil {
		return fmt.Errorf("new discord session: %w", err)
	}

	err = DiscordSession.Open()
	if err != nil {
		return fmt.Errorf("open discord session: %w", err)
	}
	return nil
}

// set status
func setDiscordStatus(status string) error {
	err := DiscordSession.UpdateCustomStatus(status)
	if err != nil {
		return fmt.Errorf("update custom status: %w", err)
	}
	return nil
}

// close session and unregister slash commands
func CloseDiscordClient() error {
	_, err := DiscordSession.ApplicationCommandBulkOverwrite(
		DiscordSession.State.Application.ID, config.EnvConfig.GuildID, []*discordgo.ApplicationCommand{})
	if err != nil {
		return fmt.Errorf("application bulk overwrite: %w", err)
	}

	// for case bot had before another commands
	_, err = DiscordSession.ApplicationCommandBulkOverwrite(
		DiscordSession.State.Application.ID, "", []*discordgo.ApplicationCommand{})
	if err != nil {
		return fmt.Errorf("application bulk overwrite: %w", err)
	}

	err = DiscordSession.Close()
	// saves if err != nil, it return nil if ok otherwise err
	if err != nil {
		return fmt.Errorf("discord session close: %w", err)
	}
	return nil
}

// register slash commands with permission for mods
func RegistryApplicationCommands() error {
	permsMod := int64(discordgo.PermissionModerateMembers)

	_, err := DiscordSession.ApplicationCommandBulkOverwrite(
		DiscordSession.State.User.ID, config.EnvConfig.GuildID, []*discordgo.ApplicationCommand{
			{
				Name:        "set-config",
				Description: "configure bot",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Name:        "playlist-link",
						Description: "link to youtube link",
						Type:        discordgo.ApplicationCommandOptionString,
						Required:    false,
					},
					{
						Name:        "set-next-song",
						Description: "Manually set position in playlist",
						Type:        discordgo.ApplicationCommandOptionInteger,
						Required:    false,
					},
					{
						Name:        "publishing-time",
						Description: "HH:MM format, sets hour when should be song published, in UTC 24h time format, default 12:00",
						Type:        discordgo.ApplicationCommandOptionString,
						Required:    false,
					},
					{
						Name:        "publishing-channel",
						Description: "Sets channel to sending daily song",
						Type:        discordgo.ApplicationCommandOptionChannel,
						Required:    false,
					},
				},
				DefaultMemberPermissions: &permsMod,
			},
			{
				Name:                     "get-config",
				Description:              "display current configuration",
				DefaultMemberPermissions: &permsMod,
			},
		})
	if err != nil {
		return fmt.Errorf("application bulk overwrite: %w", err)
	}
	return nil
}

// send respond to slash command, visible just for user sho use slash
func RespondUserEphemeral(i *discordgo.InteractionCreate, message string) {
	err := DiscordSession.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		errorlog.Logger.Error(fmt.Errorf("respond user ephemeral: %w", err).Error())
	}
}

// send song to channel, only when discord is connected
func SendDailySong(songNum int64, channelID string, playlistID string) error {
	songID, err := youtube.GetSong(playlistID, songNum)
	if err != nil {
		return fmt.Errorf("youtube get song: %w", err)
	}

	songUrl := url.URL{
		Scheme: "https",
		Host:   "youtube.com",
		Path:   "/watch",
	}
	songQuery := songUrl.Query()
	songQuery.Add("v", songID)
	songUrl.RawQuery = songQuery.Encode()

	canSend.L.Lock()
	for !isConnected.Load() {
		canSend.Wait()
	}

	_, err = DiscordSession.ChannelMessageSend(channelID, songUrl.String())
	canSend.L.Unlock()
	if err != nil {
		return fmt.Errorf("channel message send: %w", err)
	}
	return nil
}
