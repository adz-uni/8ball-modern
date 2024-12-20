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
	signingSecret := os.Getenv("SLACK_SIGNING_SECRET")

	// Validate environment variables
	if botToken == "" || signingSecret == "" {
		log.Fatal("Error: SLACK_BOT_TOKEN and SLACK_SIGNING_SECRET must be set")
	}

	// Initialize Slack client
	client := slack.New(botToken)

	// Seed the random number generator
	rand.Seed(time.Now().UnixNano())

	// Set up the HTTP handler for /ask8ball
	http.HandleFunc("/ask8ball", func(w http.ResponseWriter, r *http.Request) {
		handleSlashCommand(w, r, signingSecret, client)
	})

	// Get the port from environment variable, default to 8080
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting HTTP server on :%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Error starting HTTP server: %v", err)
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

	// Determine the response based on the input
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
		 "text": "Magic 8-ball is here! (This is a test response)",
	}

	respBytes, err := json.Marshal(response)
	if err != nil {
		log.Printf("Error marshaling response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	w.Write(respBytes)
}
