package main

import (
	"encoding/json"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

// Array of Magic 8-ball responses
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
	// Load environment variables
	botToken := os.Getenv("SLACK_BOT_TOKEN")
	appToken := os.Getenv("SLACK_APP_TOKEN")
	signingSecret := os.Getenv("SLACK_SIGNING_SECRET")

	if botToken == "" || appToken == "" || signingSecret == "" {
		log.Fatal("Error: SLACK_BOT_TOKEN, SLACK_APP_TOKEN, and SLACK_SIGNING_SECRET must be set")
	}

	// Seed the random number generator
	rand.Seed(time.Now().UnixNano())

	// Initialize Slack client with Socket Mode
	client := slack.New(
		botToken,
		slack.OptionAppLevelToken(appToken),
	)

	socketClient := socketmode.New(
		client,
		socketmode.OptionLog(log.New(os.Stdout, "socketmode: ", log.Lshortfile|log.LstdFlags)),
	)

	// Run the Socket Mode client in a separate goroutine
	go func() {
		err := socketClient.Run()
		if err != nil {
			log.Fatalf("Error running socketmode client: %v", err)
		}
	}()

	// Start HTTP server to handle slash commands
	http.HandleFunc("/ask8ball", func(w http.ResponseWriter, r *http.Request) {
		handleSlashCommand(w, r, signingSecret, client)
	})

	go func() {
		port := "8080"
		log.Printf("Starting HTTP server on :%s", port)
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			log.Fatalf("Error starting HTTP server: %v", err)
		}
	}()

	log.Println("Magic 8-ball bot is running with Socket Mode and HTTP server...")

	// Event loop for Socket Mode
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

// handleAppMention processes app_mention events
func handleAppMention(client *slack.Client, event *slackevents.AppMentionEvent) {
	log.Printf("Received mention from user %s: %s", event.User, event.Text)

	userQuery := strings.TrimSpace(event.Text)
	// Remove the bot mention from the message
	userQuery = removeBotMention(userQuery)

	log.Printf("Processed query: %s", userQuery)

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

	log.Printf("Replying with: %s", reply)

	// Send a message back to the channel
	_, _, err := client.PostMessage(event.Channel, slack.MsgOptionText(reply, false))
	if err != nil {
		log.Printf("Error sending message: %v", err)
	}
}

// handleSlashCommand processes the /ask8ball slash command
func handleSlashCommand(w http.ResponseWriter, r *http.Request, signingSecret string, client *slack.Client) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Verify the request signature
	sv, err := slack.NewSecretsVerifier(r.Header, signingSecret)
	if err != nil {
		log.Printf("Error creating secrets verifier: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if _, err := sv.Write(body); err != nil {
		log.Printf("Error writing to secrets verifier: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if err := sv.Ensure(); err != nil {
		log.Printf("Error verifying signature: %v", err)
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Parse the slash command payload
	payload, err := slack.SlashCommandParse(r)
	if err != nil {
		log.Printf("Error parsing slash command: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Respond immediately to Slack to avoid timeout
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")

	var reply string
	if !strings.HasSuffix(payload.Text, "?") {
		reply = "Where's the question?"
	} else {
		matched, err := regexp.MatchString("^(who|what|when|where|why|how|if).*", strings.ToLower(payload.Text))
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

	log.Printf("Slash Command Replying with: %s", reply)

	response := map[string]string{
		"text": reply,
	}

	respBytes, err := json.Marshal(response)
	if err != nil {
		log.Printf("Error marshaling response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Write(respBytes)
}

// removeBotMention removes the bot's mention from the message text
func removeBotMention(text string) string {
	// Mentions are usually like <@U12345>, so we find that pattern
	mentionPattern := regexp.MustCompile(`<@[^>]+>`)
	return strings.TrimSpace(mentionPattern.ReplaceAllString(text, ""))
}
