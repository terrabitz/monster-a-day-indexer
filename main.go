package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"os"
	"strings"

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

	openAIClient := NewOpenAIClient(cfg.OpenAIToken)

	postsReq, err := redditClient.NewJSONRequest("GET", "r/monsteraday/hot?limit=1", nil)
	if err != nil {
		return fmt.Errorf("couldn't create request: %w", err)
	}

	postsRes, err := redditClient.Do(context.Background(), postsReq, nil)
	if err != nil {
		return fmt.Errorf("couldn't get posts: %w", err)
	}
	defer postsRes.Body.Close()

	var listing RedditListing
	if err := json.NewDecoder(postsRes.Body).Decode(&listing); err != nil {
		return fmt.Errorf("couldn't parse response body: %w", err)
	}

	for _, post := range listing.Data.Children {
		if post.Data.MediaMetadata == nil {
			continue
		}

		galleryImageIDs := Map(post.Data.GalleryData.Items, func(i GalleryDataItem) string {
			return i.MediaID
		})

		imageURLs := Map(galleryImageIDs, func(id string) string {
			metadata := post.Data.MediaMetadata[id]
			for _, image := range metadata.PreviewImages {
				if image.Width >= 640 {
					return image.URL
				}
			}

			return metadata.OriginalImage.URL
		})

		decodedImageURLs := Map(imageURLs, func(u string) string {
			return html.UnescapeString(u)
		})

		fmt.Println(post.Data.Title)
		for _, u := range decodedImageURLs {
			fmt.Printf("- %s\n", u)
		}
		fmt.Println()

		imageParts := Map(decodedImageURLs, func(u string) GetChatCompletionRequestPart {
			return GetChatCompletionRequestPart{
				Type: "image_url",
				ImageURL: struct {
					URL string "json:\"url\""
				}{
					URL: u,
				},
			}
		})

		promptPart := GetChatCompletionRequestPart{
			Type: "text",
			Text: "This provided image is a D&D 5e monster statblock. Please read the following pieces of information from the image, and output them in CSV format without any header and each value in double-quotes: name, challenge rating, armor class, type, and size. If you cannot find a statblock, return a string starting with \"error:\"  and a short message containing the problem.",
		}

		parts := []GetChatCompletionRequestPart{}
		parts = append(parts, promptPart)
		parts = append(parts, imageParts...)

		openAIReq := GetChatCompletionRequest{
			Model:     "gpt-4-vision-preview",
			MaxTokens: 300,
			Messages: []GetChatCompletionRequestMessage{
				{
					Role:    "user",
					Content: parts,
				},
			},
		}

		res, err := openAIClient.GetChatCompletion(context.Background(), openAIReq)
		if err != nil {
			return fmt.Errorf("couldn't get OpenAI chat completions: %w", err)
		}

		chatResponse := res.Choices[0].Message.Content
		if strings.HasPrefix(chatResponse, "error:") {
			return fmt.Errorf("error while parsing image: %s", strings.TrimPrefix(chatResponse, "error:"))
		}

		csvReader := csv.NewReader(strings.NewReader(chatResponse))
		records, err := csvReader.ReadAll()
		if err != nil {
			return fmt.Errorf("error reading CSV: %w", err)
		}

		fmt.Printf("%v\n", records)
	}

	return nil
}

func printJSON(v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
}

type RedditListing struct {
	Kind string `json:"kind"`
	Data struct {
		After     string      `json:"after"`
		Dist      int         `json:"dist"`
		Modhash   interface{} `json:"modhash"`
		GeoFilter interface{} `json:"geo_filter"`
		Children  []struct {
			Kind string     `json:"kind"`
			Data RedditPost `json:"data"`
		} `json:"children"`
	} `json:"data"`
}

type RedditPost struct {
	GalleryData struct {
		Items []GalleryDataItem `json:"items"`
	} `json:"gallery_data"`
	MediaMetadata map[string]MediaMetadata `json:"media_metadata"`
	URL           string                   `json:"url"`
	Author        string                   `json:"author"`
	Title         string                   `json:"title"`
}

type GalleryDataItem struct {
	OutboundURL string `json:"outbound_url"`
	MediaID     string `json:"media_id"`
	ID          int    `json:"id"`
}

type MediaMetadata struct {
	Status        string `json:"status"`
	Type          string `json:"e"`
	MimeType      string `json:"m"`
	PreviewImages []struct {
		Height int    `json:"y"`
		Width  int    `json:"x"`
		URL    string `json:"u"`
	} `json:"p"`
	OriginalImage struct {
		Height int    `json:"y"`
		Width  int    `json:"x"`
		URL    string `json:"u"`
	} `json:"s"`
	ID string `json:"id"`
}

func Map[T, U any](tt []T, fn func(T) U) []U {
	var res []U
	for _, t := range tt {
		res = append(res, fn(t))
	}

	return res
}
