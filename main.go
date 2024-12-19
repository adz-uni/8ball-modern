package main

import (
	"log"
	"math/rand"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

/* Our array of canned responses */
var magicResponse = []string{
	"Signs point to no.",
	"Yes.",
	"Reply hazy, try again.",
	"Without a doubt.",
	"My sources say no.",
	"As I see it, yes.",
	"You may rely on it.",
	"Concentrate and ask again.",
	"Outlook not so good.",
	"It is decidedly so.",
	"Better not tell you now.",
	"Very doubtful.",
	"Yes - definitely.",
	"It is certain.",
	"Cannot predict now.",
	"Most likely.",
	"Ask again later.",
	"My reply is no.",
	"Outlook good.",
	"Don't count on it.",
}

func main() {
	botToken := os.Getenv("SLACK_BOT_TOKEN")
	appToken := os.Getenv("SLACK_APP_TOKEN")

	if botToken == "" || appToken == "" {
		log.Fatal("Error: SLACK_BOT_TOKEN and SLACK_APP_TOKEN must be set")
	}

	// Seed random number generator for random responses
	rand.Seed(time.Now().UnixNano())

	// Create a new Slack client with Socket Mode support
	client := slack.New(
		botToken,
		slack.OptionAppLevelToken(appToken),
	)

	socketClient := socketmode.New(
		client,
		socketmode.OptionLog(log.New(os.Stdout, "socketmode: ", log.Lshortfile|log.LstdFlags)),
	)

	// Run the socket mode client in a separate goroutine
	go func() {
		err := socketClient.Run()
		if err != nil {
			log.Fatalf("Error running socketmode client: %v", err)
		}
	}()

	log.Println("Magic 8-ball bot is running with Socket Mode...")

	for evt := range socketClient.Events {
		switch evt.Type {
		case socketmode.EventTypeConnecting:
			log.Println("Connecting to Slack...")
		case socketmode.EventTypeConnected:
			log.Println("Connected to Slack!")
		case socketmode.EventTypeEventsAPI:
			eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
			if !ok {
				// If you can't cast, skip this event
				socketClient.Ack(*evt.Request)
				continue
			}

			// Acknowledge the event to Slack
			socketClient.Ack(*evt.Request)

			// Handle the inner event
			switch event := eventsAPIEvent.InnerEvent.Data.(type) {
			case *slackevents.AppMentionEvent:
				handleAppMention(client, event)
			default:
				// You can handle other event types here if needed
			}
		}
	}
}

// handleAppMention responds to messages that mention the bot
func handleAppMention(client *slack.Client, event *slackevents.AppMentionEvent) {
	userQuery := strings.TrimSpace(event.Text)
	// Remove the mention text (e.g., "<@U123ABC> ") from the start
	userQuery = removeBotMention(userQuery, event.User)

	var reply string
	if !strings.HasSuffix(userQuery, "?") {
		reply = "Where's the question?"
	} else {
		matched, err := regexp.MatchString("^(who|what|when|where|why|how|if).*", strings.ToLower(userQuery))
		if err != nil {
			log.Printf("Regex error: %v", err)
			reply = "I'm having trouble understanding. Please try again."
		} else if matched {
			reply = "I'm not a tarot deck. Yes or no questions please."
		} else {
			// Provide a random response
			reply = magicResponse[rand.Intn(len(magicResponse))]
		}
	}

	// Send a message back to the channel where the mention occurred
	_, _, err := client.PostMessage(event.Channel, slack.MsgOptionText(reply, false))
	if err != nil {
		log.Printf("Error sending message: %v", err)
	}
}

// removeBotMention strips the bot mention from the message text
func removeBotMention(text, userID string) string {
	// Mentions are usually like <@U12345>, so we find that pattern
	mentionPattern := regexp.MustCompile(`<@[^>]+>`)
	return strings.TrimSpace(mentionPattern.ReplaceAllString(text, ""))
}
