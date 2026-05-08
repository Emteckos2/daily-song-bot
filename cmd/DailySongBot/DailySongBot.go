package main

import (
	"context"
	"dailysongbot/internal/commands"
	"dailysongbot/internal/config"
	"dailysongbot/internal/discord"
	"dailysongbot/internal/errorlog"
	"dailysongbot/internal/scheduler"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

func main() {
	// os term call or ctrl+c
	var (
		err       error
		ctx, stop = signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		exitInt   = 0
	)
	defer stop()

	// decode json
	err = config.ReadConfig()
	if err != nil {
		errorlog.Logger.Error(fmt.Errorf("read config: %w", err).Error())
		exitInt = 1
		stop()
	}

	// discord bot init
	err = discord.StartDiscordClient()
	if err != nil {
		errorlog.Logger.Error(fmt.Errorf("start discord client: %w", err).Error())
		exitInt = 2
		stop()
	}

	// first connect
	discord.HandleConnection()

	discord.DiscordSession.AddHandler(func(s *discordgo.Session, i *discordgo.Resumed) {
		// when discord is reconnect
		discord.HandleConnection()
	})

	discord.DiscordSession.AddHandler(func(s *discordgo.Session, i *discordgo.Disconnect) {
		discord.HandleDisconnect()
	})

	err = discord.RegistryApplicationCommands()
	if err != nil {
		errorlog.Logger.Error(fmt.Errorf("registry app commands: %w", err).Error())
		exitInt = 3
		stop()
	}

	discord.DiscordSession.AddHandler(commands.ApplicationCommandHandler)

	// run sheduler to controll publishing
	go func() {
		err = scheduler.Scheduler(ctx)
		if err != nil {
			errorlog.Logger.Error(fmt.Errorf("sheduler: %w", err).Error())
			exitInt = 4
			stop()
		}
	}()

	// called exit
	<-ctx.Done()

	err = discord.CloseDiscordClient()
	if err != nil {
		errorlog.Logger.Error(fmt.Errorf("closing discord client: %w", err).Error())
		os.Exit(5)
	}
	os.Exit(exitInt)
}
