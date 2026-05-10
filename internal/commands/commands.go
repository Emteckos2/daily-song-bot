package commands

import (
	"dailysongbot/internal/config"
	"dailysongbot/internal/discord"
	"dailysongbot/internal/errorlog"
	"dailysongbot/internal/youtube"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

// logic to call funcion for equel interactions
func ApplicationCommandHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	var err error

	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}

	data := i.ApplicationCommandData()

	switch data.Name {
	case "set-config":
		err = setConfigCommandHandler(i)

	case "get-config":
		err = getConfigCommandHandler(i)

	default:
	}

	if err != nil {
		errorlog.Logger.Error(fmt.Errorf("%s handler: %w", data.Name, err).Error())
	}
}

// check configs user input config and save them if ok
// also tell user if was done or not
func setConfigCommandHandler(i *discordgo.InteractionCreate) error {
	var (
		err error
		cfg config.UserConfigDataStruct
	)

	defer func() {
		userresponseString := "OK"
		if err != nil {
			userresponseString = "Failed, check logs"
		}
		discord.RespondUserEphemeral(i, userresponseString)
	}()

	// load configs
	if cfg, err = config.Get(); err != nil {
		return fmt.Errorf("user config load: %w", err)
	}

	// loop and switch all comands parameters user sends
	for _, d := range i.ApplicationCommandData().Options {

		switch d.Name {
		case "set-next-song":
			cfg.PlaylistNextSong = d.IntValue()

		case "publishing-time":
			var h, m int
			h, m, err = decodePostTime(d.StringValue())
			if err != nil {
				return fmt.Errorf("decoding time: %w", err)
			}
			cfg.PostHours = h
			cfg.PostMinutes = m

			cfg.NextRunTime = time.Date(cfg.NextRunTime.Year(), cfg.NextRunTime.Month(),
				cfg.NextRunTime.Day(), h, m, 0, 0, time.UTC)

		case "publishing-channel":
			cfg.ChannelID = d.ChannelValue(discord.DiscordSession).ID

		case "playlist-link":
			var (
				p      string
				result bool
			)
			p, err = parsePlaylistLink(d.StringValue())
			if err != nil {
				return fmt.Errorf("parse youtube link: %w", err)
			}
			result, err = youtube.TestPlaylist(p)
			if err != nil {
				return fmt.Errorf("test youtube link: %w", err)
			}
			if !result {
				return fmt.Errorf("test youtube link: playlist not found")
			}
			cfg.PlaylistID = p
		default:
		}
	}

	// save new config
	err = config.Save(&cfg)
	if err != nil {
		return fmt.Errorf("problem saving config: %w", err)
	}

	return nil
}

func getConfigCommandHandler(i *discordgo.InteractionCreate) error {
	var (
		err                error
		cfg                config.UserConfigDataStruct
		userresponseString string
	)

	// output for user
	defer func() {
		if err != nil {
			userresponseString = "Failed, check logs"
		}
		discord.RespondUserEphemeral(i, userresponseString)
	}()

	if cfg, err = config.Get(); err != nil {
		return fmt.Errorf("user config load: %w", err)
	}

	// using json get output
	b, err := json.MarshalIndent(cfg, "", " ")
	if err != nil {
		return fmt.Errorf("unable to encode config for user: %w", err)
	}

	// wrap text to discord format for code
	userresponseString = fmt.Sprintf("```json\n%s\n```", string(b))
	return nil
}

// make from string HH:MM ints h and m
func decodePostTime(s string) (int, int, error) {
	var (
		err  error
		h, m int
	)

	o := strings.Split(s, ":")
	if len(o) != 2 {
		return 0, 0, fmt.Errorf("decode post time: failed split string")
	}

	h, err = strconv.Atoi(o[0])
	if err != nil {
		return 0, 0, fmt.Errorf("decode hours : %w", err)
	}

	m, err = strconv.Atoi(o[1])
	if err != nil {
		return 0, 0, fmt.Errorf("decode minutes : %w", err)
	}

	return h, m, nil
}

// get id of playlist from link
func parsePlaylistLink(p string) (string, error) {
	u, err := url.Parse(p)
	if err != nil {
		return "", fmt.Errorf("url parse: %w", err)
	}

	// accept only theese hostnames
	switch u.Hostname() {
	case "youtube.com", "youtu.be", "music.youtube.com":
	default:
		return "", fmt.Errorf("parse yt playlist: not supported domain")
	}

	// must contain playlist in url path
	if u.Path != "/playlist" {
		return "", fmt.Errorf("parse yt playlist: not youtube playlist link")
	}

	// load id
	q := u.Query()
	playlistId := q.Get("list")
	if playlistId == "" {
		return "", fmt.Errorf("parse yt playlist: no youtube playlist id")
	}

	return playlistId, nil
}
