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

	"github.com/bwmarrin/discordgo"
)

func ApplicationCommandHandler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	var err error

	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		data := i.ApplicationCommandData()

		switch data.Name {
		case "set-config":
			err = setConfigCommandHandler(i)
		case "get-config":
			err = getConfigCommandHandler(i)
		default:
		}
	default:
	}

	if err != nil {
		errorlog.Logger.Error(fmt.Errorf("aplication interaction handler: %w", err).Error())
	}
}

func setConfigCommandHandler(i *discordgo.InteractionCreate) error {
	userResponceString := "Failed"
	defer func() {
		discord.RespondUserEphemeral(i, userResponceString)
	}()
	// users data
	a := i.ApplicationCommandData()
	// map of problems to show user what they fuckup
	errs := make(map[string]error)

	// commands names
	setNextSong, publishingTime, publishingChannel, playlistLink :=
		"set-next-song", "publishing-time", "publishing-channel", "playlist-link"

	// make copy of config to compare if was changed

	// loop and switch all comands parameters user sends
	for _, d := range a.Options {
		switch d.Name {

		case setNextSong:
			if d.IntValue() > 0 {
				if ud, ok := config.UserConfig.Load().(config.UserConfigDataStruct); ok {
					ud.PlaylistNextSong = d.IntValue()
					config.UserConfig.Store(ud)
				} else {
					errs[setNextSong] = fmt.Errorf("set next song: user config load: nil data")
					errorlog.Logger.Warn(fmt.Errorf("set-config handler: %w", errs[setNextSong]).Error())
				}
			} else {
				errs[setNextSong] = fmt.Errorf("set next song: negative playlist position")
				errorlog.Logger.Warn(fmt.Errorf("set-config handler: %w", errs[setNextSong]).Error())
			}

		case publishingTime:
			var h, m int
			h, m, errs[publishingTime] = decodePostTime(d.StringValue())
			if errs[publishingTime] != nil {
				errorlog.Logger.Warn(fmt.Errorf("set-config handler: %w", errs[publishingTime]).Error())
				break
			}

			if ud, ok := config.UserConfig.Load().(config.UserConfigDataStruct); ok {
				ud.PostHours = h
				ud.PostMinutes = m
				config.UserConfig.Store(ud)
			} else {
				errs[publishingTime] = fmt.Errorf("publishing time: user config load: nil data")
				errorlog.Logger.Warn(fmt.Errorf("set-config handler: %w", errs[publishingTime]).Error())
			}

		case publishingChannel:
			if ud, ok := config.UserConfig.Load().(config.UserConfigDataStruct); ok {
				ud.ChannelID = d.ChannelValue(discord.DiscordSession).ID
				config.UserConfig.Store(ud)
			} else {
				errs[publishingChannel] = fmt.Errorf("publishing channel: user config load: nil data")
				errorlog.Logger.Warn(fmt.Errorf("set-config handler: %w", errs[publishingChannel]).Error())
			}

		case playlistLink:
			var (
				p      string
				result bool
			)
			p, errs[playlistLink] = parslePlaylistLink(d.StringValue())
			if errs[playlistLink] != nil {
				errorlog.Logger.Warn(fmt.Errorf("parsle youtube link: %w", errs[playlistLink]).Error())
				break
			}
			result, errs[playlistLink] = youtube.TestPlaylist(p)
			if errs[playlistLink] != nil {
				errorlog.Logger.Warn(fmt.Errorf("youtube test link: %w", errs[playlistLink]).Error())
				break
			}
			if !result {
				errs[playlistLink] = fmt.Errorf("playlist wasnt found on youtube")
				errorlog.Logger.Warn(fmt.Errorf("set-config handler: %w", errs[playlistLink]).Error())
				break
			}

			if ud, ok := config.UserConfig.Load().(config.UserConfigDataStruct); ok {
				ud.PlaylistID = p
				config.UserConfig.Store(ud)
			} else {
				errs[playlistLink] = fmt.Errorf("playlist link: user config load: nil data")
				errorlog.Logger.Warn(fmt.Errorf("set-config handler: %w", errs[playlistLink]).Error())
			}
		default:
		}
	}

	// write what was ware changed
	e := config.WriteConfig()
	if e != nil {
		errs["write-config"] = e
		return fmt.Errorf("set-config handler: problem saving config: %w", e)
	}

	// create list of errors for user
	respond := strings.Builder{}
	for s := range errs {
		if errs[s] != nil {
			respond.WriteString(errs[s].Error())
			// add newline, faster then fmt operations
			respond.WriteByte('\n')
		}
	}

	if respond.Len() != 0 {
		userResponceString = respond.String()
	} else {
		userResponceString = "Everything OK, saved"
	}

	return nil
}

func getConfigCommandHandler(i *discordgo.InteractionCreate) error {
	userResponceString := "Failed"
	defer func() {
		discord.RespondUserEphemeral(i, userResponceString)
	}()

	resp := strings.Builder{}
	data := make(map[string]any)

	if ud, ok := config.UserConfig.Load().(config.UserConfigDataStruct); ok {
		b, err := json.Marshal(ud)
		if err != nil {
			return fmt.Errorf("json marshall: unable to encode config for user: %w", err)
		}

		err = json.Unmarshal(b, &data)
		if err != nil {
			return fmt.Errorf("get-config handler: unable to decode config for user: %w", err)
		}
	} else {
		return fmt.Errorf("user config load: nil data")
	}

	for s := range data {
		val := fmt.Sprint(data[s])
		resp.WriteString(s)
		resp.WriteString(" : ")
		resp.WriteString(val)
		resp.WriteByte('\n')
	}

	userResponceString = resp.String()
	return nil
}

// make from string HH:MM ints h and m
func decodePostTime(s string) (int, int, error) {
	var (
		err  error
		h, m int
	)

	o := strings.Split(s, ":")
	if len(o) == 1 {
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

func parslePlaylistLink(p string) (string, error) {
	u, err := url.Parse(p)
	if err != nil {
		return "", fmt.Errorf("url parse: %w", err)
	}

	switch u.Hostname() {
	case "youtube.com", "www.youtube.com", "youtu.be", "music.youtube.com":
		// everything ok, continue
	default:
		return "", fmt.Errorf("parsle yt playlist: not supported domain")
	}

	if u.Path != "/playlist" {
		return "", fmt.Errorf("parsle yt playlist: not youtube playlist link")
	}

	if !u.Query().Has("list") {
		return "", fmt.Errorf("parsle yt playlist: no youtube playlist id")
	}

	q := u.Query()
	playlistId := q.Get("parsle yt playlist: list")

	return playlistId, nil
}
