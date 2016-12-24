package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/oauth2"
)

var (
	err      error
	confFile string
	config   *TConfig

	chanHUP = make(chan os.Signal, 1)
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: " + os.Args[0] + " <config_file>")
	}

	confFile = os.Args[1]
	getConfig(confFile)

	signal.Notify(chanHUP, syscall.SIGHUP)
	go waitHUP(chanHUP)

	oauth2.RegisterBrokenAuthHeaderProvider(config.TokenURL)

	tok, err := config.Oauth2Config.PasswordCredentialsToken(oauth2.NoContext, config.Username, config.Password)
	if err != nil {
		log.Fatal(err)
	}

	err = setCache(config.AccessTokenPath, tok.AccessToken)
	if err != nil {
		log.Fatal(err)
	}

	tokenSource := config.Oauth2Config.TokenSource(oauth2.NoContext, tok)

	for {
		ctok, err := getCache(config.AccessTokenPath)
		if err != nil {
			log.Fatal(err)
		}

		ntok, err := tokenSource.Token()
		if err != nil {
			log.Fatal(err)
		}

		if ntok.AccessToken != ctok {
			err := setCache(config.AccessTokenPath, ntok.AccessToken)
			if err != nil {
				log.Fatal(err)
			}
		}

		time.Sleep(1 * time.Minute)
	}
}
