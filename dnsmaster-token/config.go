package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"

	"golang.org/x/oauth2"
)

type (
	TConfig struct {
		ClientID        string   `json:"client_id"`
		ClientSecret    string   `json:"client_secret"`
		Username        string   `json:"username"`
		Password        string   `json:"password"`
		Scope           []string `json:"scope"`
		TokenURL        string   `json:"token_url"`
		Oauth2Config    *oauth2.Config
		AccessTokenPath string `json:"access_token_path"`
	}
)

func getConfig(file_path string) {
	f, err := os.Open(file_path)
	if err != nil {
		log.Fatal("error:", err)
		return
	}
	defer f.Close()

	decoder := json.NewDecoder(f)
	err = decoder.Decode(&config)
	if err != nil {
		log.Fatal("error:", err)
	}

	config.Oauth2Config = &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		Scopes:       config.Scope,
		Endpoint: oauth2.Endpoint{
			TokenURL: config.TokenURL,
		},
	}
}

func waitHUP(c chan os.Signal) {
	for {
		<-c

		getConfig(confFile)
	}
}

func getCache(file_path string) (string, error) {
	f, err := os.OpenFile(file_path, os.O_RDONLY, 0600)
	if err != nil {
		return "", err
	}
	defer f.Close()

	contents, err := ioutil.ReadAll(f)
	if err != nil {
		return "", err
	}

	return string(contents), nil
}

func setCache(file_path, token string) error {
	f, err := os.OpenFile(file_path, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write([]byte(token))
	if err != nil {
		return err
	}

	return nil
}
