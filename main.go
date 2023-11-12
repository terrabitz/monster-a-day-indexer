package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/caarlos0/env/v10"
	"github.com/joho/godotenv"
	"github.com/vartanbeno/go-reddit/v2/reddit"
	"github.com/xlzd/gotp"
)

type Config struct {
	OpenAIToken        string `env:"OPENAI_TOKEN,required"`
	RedditUsername     string `env:"REDDIT_USERNAME,required"`
	RedditPassword     string `env:"REDDIT_PASSWORD,required"`
	RedditClientID     string `env:"REDDIT_CLIENT_ID,required"`
	RedditClientSecret string `env:"REDDIT_CLIENT_SECRET,required"`
	RedditTOTPSecret   string `env:"REDDIT_TOTP_SECRET,required"`
}

func main() {
	if err := run(); err != nil {
		log.Printf("got eror: %v", err)
		os.Exit(1)
	}
}

func run() error {
	godotenv.Load()

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return fmt.Errorf("error loading environment variables: %w", err)
	}

	totp := gotp.NewDefaultTOTP(cfg.RedditTOTPSecret)
	totpSecret := totp.Now()

	redditCredentials := reddit.Credentials{
		ID:       cfg.RedditClientID,
		Secret:   cfg.RedditClientSecret,
		Username: cfg.RedditUsername,
		Password: fmt.Sprintf("%s:%s", cfg.RedditPassword, totpSecret),
	}

	redditClient, err := reddit.NewClient(redditCredentials)
	if err != nil {
		return fmt.Errorf("couldn't create Reddit client: %w", err)
	}

	sub, _, err := redditClient.Subreddit.Get(context.Background(), "monsteraday")
	if err != nil {
		return fmt.Errorf("couldn't get subreddit info: %w", err)
	}

	printJSON(sub)

	return nil
}

func printJSON(v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
}
