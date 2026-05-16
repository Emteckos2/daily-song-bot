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

	// read configs, close when error or new config is created
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

	// first bot connect
	discord.HandleConnection()

	// when discord is reconnect
	discord.DiscordSession.AddHandler(func(s *discordgo.Session, i *discordgo.Resumed) {
		discord.HandleConnection()
	})

	// call when connection end
	discord.DiscordSession.AddHandler(func(s *discordgo.Session, i *discordgo.Disconnect) {
		discord.HandleDisconnect()
	})

	// registry slash commands
	err = discord.RegistryApplicationCommands()
	if err != nil {
		errorlog.Logger.Error(fmt.Errorf("registry app commands: %w", err).Error())
		exitInt = 3
		stop()
	}

	// interaction on slash command
	discord.DiscordSession.AddHandler(commands.ApplicationCommandHandler)

	// run scheduler to control publishing
	go func() {
		err = scheduler.Scheduler(ctx)
		if err != nil {
			errorlog.Logger.Error(fmt.Errorf("scheduler: %w", err).Error())
			exitInt = 4
			stop()
		}
	}()

	// called exit, user or system
	<-ctx.Done()

	// peacefully close discord
	err = discord.CloseDiscordClient()
	if err != nil {
		errorlog.Logger.Error(fmt.Errorf("closing discord client: %w", err).Error())
		os.Exit(5)
	}
	// if no problem, exitInt is 0
	os.Exit(exitInt)
}
