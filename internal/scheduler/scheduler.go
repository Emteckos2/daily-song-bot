package scheduler

import (
	"context"
	"dailysongbot/internal/config"
	"dailysongbot/internal/discord"
	"dailysongbot/internal/errorlog"
	"dailysongbot/internal/youtube"
	"errors"
	"fmt"
	"time"
)

func Scheduler(ctx context.Context) error {
	for {
		// dont run if is cancelled on start
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		// adds 1 day, it can work even with time shift
		var (
			ok  bool
			ud  config.UserConfigDataStruct
			err error
		)
		if ud, ok = config.UserConfig.Load().(config.UserConfigDataStruct); !ok {
			return fmt.Errorf("user config load: nil data")
		}

		err = config.CheckConfigSettings(&ud)
		if err != nil {
			errorlog.Logger.Warn("config check setting: " + err.Error())
			sleepWithContext(ctx, time.Minute)
			if ctx.Err() != nil {
				return nil
			}
			continue
		}

		last := ud.LastRunTime.UTC()
		next := time.Date(last.Year(), last.Month(), last.Day(),
			ud.PostHours, ud.PostMinutes, 0, 0, time.UTC).AddDate(0, 0, 1)

		waitDuration := time.Until(next)
		timer := time.NewTimer(waitDuration)

		select {
		// park gorutine until time come
		case <-timer.C:
			timer.Stop()
			// load fresh infos
			if ud, ok = config.UserConfig.Load().(config.UserConfigDataStruct); !ok {
				return fmt.Errorf("user config load: nil data")
			}

			err = config.CheckConfigSettings(&ud)
			if err != nil {
				errorlog.Logger.Warn("config check setting: " + err.Error())
				continue
			}

			channelID := ud.ChannelID
			playlistID := ud.PlaylistID

			// send next song
			err = discord.SendDailySong(ud.PlaylistNextSong,
				channelID, playlistID)

			if err != nil {
				if !errors.Is(err, youtube.ErrEndPlaylist) {
					return fmt.Errorf("send daily song: %w", err)
				}

				ud.PlaylistNextSong = 1

				err = discord.SendDailySong(ud.PlaylistNextSong,
					channelID, playlistID)

				if err != nil {
					return fmt.Errorf("send daily song: %w", err)
				}
			}

			ud.LastRunTime = time.Now().UTC()
			ud.PlaylistNextSong++

			config.UserConfig.Store(ud)
			err = config.WriteConfig()
			if err != nil {
				return fmt.Errorf("write config: %w", err)
			}

		// piecefully exit
		case <-ctx.Done():
			timer.Stop()
			return nil
		}
	}
}

func sleepWithContext(ctx context.Context, sleep time.Duration) {
	timer := time.NewTimer(sleep)
	defer timer.Stop()

	select {
	case <-timer.C:
	case <-ctx.Done():
	}
}
