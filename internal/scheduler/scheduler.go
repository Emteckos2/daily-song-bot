package scheduler

import (
	"context"
	"dailysongbot/internal/config"
	"dailysongbot/internal/discord"
	"dailysongbot/internal/youtube"
	"errors"
	"fmt"
	"time"
)

func Scheduler(ctx context.Context) error {
	// dont run if is cancelled on start
	select {
	case <-ctx.Done():
		return nil
	default:
	}

	// adds 1 day, it can work even with time shift
	var (
		cfg config.UserConfigDataStruct
		err error
	)

	// synch time for ticker
	select {
	// fire after seconds are 00
	case <-time.After(time.Until(time.Now().Truncate(time.Minute).Add(1 * time.Minute))):

	// piecefully exit if called
	case <-ctx.Done():
		return nil
	}

	// one minute interval
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		// checks if should be send
		case <-ticker.C:
			if cfg, err = config.Get(); err != nil {
				return fmt.Errorf("user config load: %w", err)
			}

			if time.Now().Before(cfg.NextRunTime) {
				continue
			}

			// send next song
			err = discord.SendDailySong(cfg.PlaylistNextSong,
				cfg.ChannelID, cfg.PlaylistID)

			if err != nil {
				if !errors.Is(err, youtube.ErrEndPlaylist) {
					return fmt.Errorf("send daily song: %w", err)
				}

				cfg.PlaylistNextSong = 1

				err = discord.SendDailySong(cfg.PlaylistNextSong,
					cfg.ChannelID, cfg.PlaylistID)

				if err != nil {
					return fmt.Errorf("send daily song: %w", err)
				}
			}

			now := time.Now()
			cfg.NextRunTime = time.Date(now.Year(), now.Month(), now.Day(),
				cfg.PostHours, cfg.PostMinutes, 0, 0, time.UTC).AddDate(0, 0, 1)
			cfg.PlaylistNextSong++

			err = config.Save(&cfg)
			if err != nil {
				return fmt.Errorf("write config: %w", err)
			}

		// piecefully exit if called
		case <-ctx.Done():
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
