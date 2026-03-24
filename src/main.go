package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"rmm26/src/aaa"
	"rmm26/src/conf"
	"rmm26/src/db"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	var (
		cfg        conf.Config
		err        error
		redisCli   *db.Client
		ldapServer *aaa.LDAPServer
		sigs       chan os.Signal
	)

	// Configure logger
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Load configuration
	cfg, err = conf.LoadConfig("etc/config.json")
	switch {
	case err != nil:
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Initialize Redis client
	redisCli, err = db.NewClient(cfg.DB)
	switch {
	case err != nil:
		log.Fatal().Err(err).Msg("Failed to initialize Redis client")
	}
	defer redisCli.Close()

	// Create RediSearch index
	err = redisCli.CreateIndex(context.Background(), "idx:dn")
	switch {
	case err != nil:
		log.Error().Err(err).Msg("Failed to create index, search might not work")
	}

	// Initialize and start LDAP server
	ldapServer = aaa.NewLDAPServer(redisCli)
	go func() {
		var (
			listenErr error
		)
		listenErr = ldapServer.ListenAndServe("0.0.0.0:389")
		switch {
		case listenErr != nil:
			log.Fatal().Err(listenErr).Msg("LDAP server failed")
		}
	}()

	// Wait for termination signal
	sigs = make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs

	log.Info().Msg("Shutting down")
}
