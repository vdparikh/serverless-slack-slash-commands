package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/eawsy/aws-lambda-go-net/service/lambda/runtime/net"
	"github.com/eawsy/aws-lambda-go-net/service/lambda/runtime/net/apigatewayproxy"
	"github.com/gorilla/mux"
)

// Constants for where the response should go
// By default it is set to ResponseInChannel
const (
	ResponseInChannel = "in_channel"
	ResponseEphemeral = "ephemeral"
)

// SlackRequest represents an incoming slash command request
type SlackRequest struct {
	Token       string
	TeamID      string
	TeamDomain  string
	ChannelID   string
	ChannelName string
	UserID      string
	UserName    string
	Command     string
	Text        string
	ResponseURL string
}

// SlackResponse represents a response to slash command
type SlackResponse struct {
	ResponseType string       `json:"response_type"`
	Text         string       `json:"text"`
	Attachments  []Attachment `json:"attachments"`
}

// Attachment is Slack attachment for slash Response
type Attachment struct {
	Fallback      string   `json:"fallback"`
	Text          string   `json:"text"`
	MarkdownIn    []string `json:"mrkdwn_in,omitempty"`
	Color         string   `json:"color,omitempty"`
	AuthorName    string   `json:"author_name,omitempty"`
	AuthorSubname string   `json:"author_subname,omitempty"`
	AuthorLink    string   `json:"author_link,omitempty"`
	AuthorIcon    string   `json:"author_icon,omitempty"`
	Title         string   `json:"title,omitempty"`
	TitleLink     string   `json:"title_link,omitempty"`
	Pretext       string   `json:"pretext,omitempty"`
	ImageURL      string   `json:"image_url,omitempty"`
	ThumbURL      string   `json:"thumb_url,omitempty"`
	Fields        []Field  `json:"fields,omitempty"`
	Footer        string   `json:"footer,omitempty"`
	FooterIcon    string   `json:"footer_icon,omitempty"`
	Timestamp     int64    `json:"ts,omitempty"`
}

// Field is a field attachment
type Field struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

// Words to map data from urban dict
type Words struct {
	Tags       []string `json:"tags"`
	ResultType string   `json:"result_type"`
	List       []Word   `json:"list"`
	Sounds     []string `json:"sounds"`
}

// Word object
type Word struct {
	Definition  string `json:"definition"`
	Permalink   string `json:"permalink"`
	ThumbsUp    int    `json:"thumbs_up"`
	Author      string `json:"author"`
	Word        string `json:"word"`
	Defid       int    `json:"defid"`
	CurrentVote string `json:"current_vote"`
	Example     string `json:"example"`
	ThumbsDown  int    `json:"thumbs_down"`
}

// Handle ... AWS Handler called by Lambda
var Handle apigatewayproxy.Handler

// Timestamp formats a time.Time into a unix epoch for Attachment
func Timestamp(t time.Time) int64 {
	return t.UTC().Unix()
}

// Utility function to response with JSON and setting header
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	// Even if you enable CORS in API Gateway, the integration response aka the API response from Lambda
	// needs to return the headers for it to work
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

	// Set response status code
	w.WriteHeader(code)

	// Encode and reply
	json.NewEncoder(w).Encode(payload)
}

func processSlackCommand(w http.ResponseWriter, r *http.Request) {

	slackRequest := &SlackRequest{}
	slackRequest.Token = r.FormValue("token")
	slackRequest.TeamID = r.FormValue("team_id")
	slackRequest.TeamDomain = r.FormValue("team_domain")
	slackRequest.ChannelID = r.FormValue("channel_id")
	slackRequest.ChannelName = r.FormValue("channel_name")
	slackRequest.UserID = r.FormValue("user_id")
	slackRequest.UserName = r.FormValue("user_name")
	slackRequest.Command = r.FormValue("command")
	slackRequest.Text = r.FormValue("text")
	slackRequest.ResponseURL = r.FormValue("response_url")

	log.Println(slackRequest)

	resp, err := slackRequest.Execute()
	if err != nil {
		respondWithJSON(w, http.StatusInternalServerError, nil)
		return
	}
	respondWithJSON(w, http.StatusOK, resp)
}

// Execute the command
// Only /define and /random are valid commands right now
func (req *SlackRequest) Execute() (*SlackResponse, error) {
	slackResponse := &SlackResponse{}
	slackResponse.ResponseType = ResponseInChannel

	var UDURL = "http://api.urbandictionary.com/v0/define?term=" + strings.Replace(req.Text, " ", "", -1)
	if req.Command == "/random" {
		UDURL = "http://api.urbandictionary.com/v0/random"
	}

	resp, err := http.Get(UDURL)
	if err != nil {
		return slackResponse, err
	}

	defer resp.Body.Close()

	data, _ := ioutil.ReadAll(resp.Body)
	if err != nil {
		return slackResponse, err
	}

	var words Words
	err = json.Unmarshal([]byte(string(data)), &words)
	if err != nil {
		return slackResponse, err
	}

	var selectedWord Word

	for _, element := range words.List {
		if element.ThumbsUp > selectedWord.ThumbsUp {
			selectedWord = element
		}
	}

	var attachements []Attachment

	var attachment Attachment
	attachment.Color = "#1D2439"

	attachment.AuthorName = selectedWord.Author
	attachment.AuthorLink = selectedWord.Permalink

	attachment.Pretext = "*" + selectedWord.Word + "*\n" + selectedWord.Definition
	attachment.Title = "Example Usage:"
	attachment.Text = selectedWord.Example

	attachment.Footer = "SSE API"
	attachment.Timestamp = Timestamp(time.Now())

	attachment.MarkdownIn = []string{"text", "pretext"}

	slackResponse.Attachments = append(attachements, attachment)

	return slackResponse, nil
}

func init() {

	// Handler setup
	ln := net.Listen()
	Handle = apigatewayproxy.New(ln, []string{"image/png"}).Handle

	// MUX routing for the API calls
	r := mux.NewRouter()

	// Slack Slash commands API
	r.Path("/slack").Methods("POST").HandlerFunc(processSlackCommand)

	go http.Serve(ln, r)
}

func main() {
	// Do Nothing
}
