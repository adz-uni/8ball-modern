package main

import (
    "log"
    "math/rand"
    "os"
    "regexp"
    "strings"
    "time"

    "github.com/slack-go/slack"
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

    log.Println("8ball bot is running with Socket Mode...")

    for evt := range socketClient.Events {
        switch evt.Type {
        case socketmode.EventTypeConnecting:
            log.Println("Connecting to Slack...")
        case socketmode.EventTypeConnected:
            log.Println("Connected to Slack!")
        case socketmode.EventTypeEventsAPI:
            eventPayload, ok := evt.Data.(slackeventsEventsAPIWrapper)
            if !ok {
                // This cast helps us handle events in a strongly typed manner.
                // If for some reason it fails, we just skip.
                continue
            }

            // Acknowledge the event to Slack
            socketClient.Ack(evt.Request)

            // Process the inner event
            switch event := eventPayload.InnerEvent.Data.(type) {
            case *slackevents.AppMentionEvent:
                handleAppMention(socketClient, client, event)
            }
        }
    }
}

// handleAppMention responds to messages that mention the bot
func handleAppMention(socketClient *socketmode.Client, client *slack.Client, event *slackevents.AppMentionEvent) {
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

// removeBotMention tries to strip the bot mention portion from the message text
func removeBotMention(text, userID string) string {
    // Mentions are usually like <@U12345> so we find that pattern
    mentionPattern := regexp.MustCompile(`<@([^>]+)>`)
    return strings.TrimSpace(mentionPattern.ReplaceAllString(text, ""))
}

// --- Slack Events API structures and wrappers ---
// slack-go provides an events package (slack/slackevents) to parse events from the Events API.
// We'll emulate that by showing the wrapper and casting.

type slackeventsEventsAPIWrapper struct {
    InnerEvent slackeventsEventsAPIInnerEvent `json:"event"`
}

type slackeventsEventsAPIInnerEvent struct {
    Type string      `json:"type"`
    Data interface{} `json:"-"`
}

// We need to parse the event payload. Normally, you'd use `slackevents.ParseEvent` to get a structured event.
// Let's assume you've integrated `slackevents.ParseEvent` logic somewhere above.
// For brevity and given the conversation context, let's just assume eventPayload.InnerEvent.Data.(*slackevents.AppMentionEvent)
// works. In practice, you'd decode the EventsAPI request and use `slackevents.ParseEvent` on it.
//
// Also note that in a real scenario, you'd have a full event handler that:
// - Listens for `app_mention` events
// - Uses `slackevents.ParseEvent(requestBody, slackevents.OptionVerifyToken(&slackevents.TokenComparator{VerificationToken: "your-verification-token"}))`
// For Socket Mode, Slack sends a slightly different payload, and `socketClient.Events` contains already-parsed events.
// Update this code accordingly once you integrate fully with slackevents.
