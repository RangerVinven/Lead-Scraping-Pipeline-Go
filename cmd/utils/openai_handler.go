package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

func GenerateAbstracts(markdownPages []string) []string {
	var prompt = `
You're provided a Markdown scrape of a website page. Your task is to provide a two-paragraph abstract of what this page is about.

Return in this JSON format:

{"abstract":"your abstract goes here"}

Rules:
- Your abstract should be comprehensive—similar level of detail as an academic abstract.
- Use a straightforward, spartan tone of voice.
- If the page has no content, return: {"abstract": "no content"}
`

	var abstracts = []string{}

	for _, markdown := range markdownPages {
		client := openai.NewClient(
			option.WithAPIKey(os.Getenv("OPENAI_API_KEY")), // defaults to os.LookupEnv("OPENAI_API_KEY")
		)
		chatCompletion, err := client.Chat.Completions.New(context.TODO(), openai.ChatCompletionNewParams{
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.SystemMessage(prompt),
				openai.UserMessage(markdown),
			},
			Model: openai.ChatModelGPT4_1Nano,
		})
		if err != nil {
			panic(err.Error())
		}

		var response = chatCompletion.Choices[0].Message.Content

		abstracts = append(abstracts, response)
	}

	return abstracts
}

func GenerateIcebreaker(abstracts []string) string {
	var combinedAbstracts = strings.Join(abstracts, "\n---\n")

// 	var prompt = `
// We just scraped a series of web pages for a business. Your task is to take their summaries and turn them into catchy, personalized openers for a cold email campaign to imply the rest of the campaign is personalised.
//
// You'll return your icebreakers in the following JSON format:
//
// {"icebreaker": "Hey {name},\n\nLove {thing}—also doing/like/a fan of {otherThing}. Wanted to run something by you.\n\nHope you'll forgive me, but I checked out your site quite a bit. I can see that {anotherThing} is important to you guys (or at least I'm assuming this given the focus on {fourthThing}). You seem like an awesome company, which means you probably get a lot of clients. I know it can be hard to service so many clients, which is what I'm here for."}
//
// Icebreaker Rules:
// - Use a spartan/laconic tone.
// - Follow the format exactly.
// - Shorten company and location names when possible.
// - Avoid obvious compliments. Focus on small, unique details.
// - If you don't know the owner's name, just put "Hey,"
// - Talk in first and second person only. Say "I" and "you", never "their".
//
// Example output:
//
// {"icebreaker": "Hey Aina,\n\nLove what you're doing at Maki-also a fan of how you make it easy for folks to reach out directly. Wanted to run something by you.\n\nI hope you'll forgive me, but I checked out your website quite a bit. I can see that discretion is important to you guys (or at least I'm assuming this given the part on your website about white-labeling your services).\n\nYou seem like an awesome company, which means you probably get a lot of clients. I know it can be hard to service so many clients, which is what I'm here for."}
// `


	var prompt = `
We just scraped a series of web pages for a business. Your task is to take their summaries and turn them into catchy, personalized openers for a cold email campaign to imply the rest of the campaign is personalised.

You'll return your icebreakers in the following JSON format:

{"icebreaker": "Hey {name},\n\nLove {thing}—also doing/like/a fan of {otherThing}. Wanted to run something by you.\n\nHope you'll forgive me, but I checked out your site quite a bit. I can see that {anotherThing} is important to you guys (or at least I'm assuming this given the focus on {fourthThing}). You seem like an awesome company, which means you probably get a lot of clients."}

Icebreaker Rules:
- Use a spartan/laconic tone.
- Follow the format exactly.
- Shorten company and location names when possible.
- Avoid obvious compliments. Focus on small, unique details.
- If you don't know the owner's name, just put "Hey,"
- Talk in first and second person only. Say "I" and "you", never "their".

Example output:

{"icebreaker": "Hey Aina,\n\nLove what you're doing at Maki-also a fan of how you make it easy for folks to reach out directly. Wanted to run something by you.\n\nI hope you'll forgive me, but I checked out your website quite a bit. I can see that discretion is important to you guys (or at least I'm assuming this given the part on your website about white-labeling your services).\n\nYou seem like an awesome company, which means you probably get a lot of clients."}
`
	
	client := openai.NewClient(
		option.WithAPIKey(os.Getenv("OPENAI_API_KEY")), // defaults to os.LookupEnv("OPENAI_API_KEY")
	)
	chatCompletion, err := client.Chat.Completions.New(context.TODO(), openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(prompt),
			openai.UserMessage(combinedAbstracts),
		},
		Model: openai.ChatModelGPT4_1Nano,
	})
	if err != nil {
		panic(err.Error())
	}

	var jsonStr = chatCompletion.Choices[0].Message.Content
	
	var data map[string]interface{}
	err = json.Unmarshal([]byte(jsonStr), &data)
	if err != nil {
		fmt.Println("Couldn't parse the JSON. Received error:", err)
		return ""
	}

	icebreaker, ok := data["icebreaker"].(string)
	if !ok {
		fmt.Println("Couldn't parse the JSON. Received error:", err)
		return ""
	}
	return icebreaker
}
