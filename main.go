package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/AssemblyAI/assemblyai-go-sdk"
	"github.com/kkdai/youtube/v2"
)

const OLLAMA_CHAT_URL = "http://localhost:11434/api/chat"

type Role string

const (
	System    Role = "system"
	User      Role = "user"
	Assistant Role = "assistant"
)

type chatRequest struct {
	Model   string    `json:"model"`
	Message []message `json:"messages"`
}

type message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

type ollamaResponse struct {
	Model                string  `json:"model"`
	Created_at           string  `json:"created_at"`
	Message              message `json:"message,omitempty"`
	Done                 bool    `json:"done"`
	Context              []int   `json:"context,omitempty"`
	Total_duration       int     `json:"total_duration,omitempty"`
	Load_duration        int     `json:"load_duration,omitempty"`
	Prompt_eval_count    int     `json:"prompt_eval_count,omitempty"`
	Prompt_eval_duration int     `json:"prompt_eval_duration,omitempty"`
	Eval_count           int     `json:"eval_count,omitempty"`
	Eval_duration        int     `json:"eval_duration,omitempty"`
}

func readPrompt() string {
	fmt.Print("> ")

	var prompt string
	read := bufio.NewReader(os.Stdin)
	prompt, err := read.ReadString('\n')
	if err != nil {
		fmt.Println("Error reading prompt:", err)
		return ""
	}
	return strings.TrimSpace(prompt)
}

func downloadVideo(videoID string, c *youtube.Client) string {
	log.Println("Downloading video...")
	video, err := c.GetVideo(videoID)
	if err != nil {
		panic(err)
	}
	filePath := video.Title + ".mp4"
	_, err = os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("File does not exist, downloading...")
			formats := video.Formats.WithAudioChannels()
			stream, _, err := c.GetStream(video, &formats[0])
			if err != nil {
				panic(err)
			}
			defer stream.Close()

			file, err := os.Create(video.Title + ".mp4")
			if err != nil {
				panic(err)
			}
			defer file.Close()

			_, err = io.Copy(file, stream)
			if err != nil {
				panic(err)
			}
			return video.Title + ".mp4"
		}
	}
	log.Println("File already exists, skipping download...")
	return video.Title + ".mp4"
}

func extractYouTubeID(url string) (string, error) {
	pattern := `(?:youtube\.com\/watch\?v=|youtu.be\/)([^&\?]+)`
	re := regexp.MustCompile(pattern)
	match := re.FindStringSubmatch(url)
	if len(match) < 2 {
		return "", fmt.Errorf("no YouTube video ID found in the URL")
	}
	return match[1], nil
}

func generateResponse(c chatRequest) {
	dat, err := json.Marshal(c)

	if err != nil {
		fmt.Println("Error marshaling JSON:", err)
	}

	req, err := http.NewRequest("POST", OLLAMA_CHAT_URL, bytes.NewBuffer(dat))
	if err != nil {
		fmt.Println("Error creating HTTP request:", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
	}
	defer resp.Body.Close()
	reader := bufio.NewReader(resp.Body)
	var res strings.Builder
	ch := make(chan string)
	go func() {
		defer close(ch)
		for {
			chunk, err := reader.ReadBytes('\n')
			if err != nil {
				if err.Error() == "EOF" {
					fmt.Println()
					break
				}
			}
			var ollamaResponse ollamaResponse
			err = json.Unmarshal(chunk, &ollamaResponse)
			if err != nil {
				fmt.Println(err)
			}
			resp := ollamaResponse.Message.Content
			res.WriteString(resp)
			ch <- resp
		}
	}()

	for resp := range ch {
		fmt.Print(resp)
	}
}

func main() {

	fmt.Println("++++++++++++++++++++==++++++++++++++++++++++++++++++++++++")
	log.Println("Welcome to YT Chat!")
	fmt.Print("Give me a YouTube video URL: ")

	var videoURL string
	vid := bufio.NewReader(os.Stdin)
	videoURL, err := vid.ReadString('\n')
	if err != nil {
		fmt.Println("Error reading prompt:", err)
	}
	videoURL = strings.TrimSpace(videoURL)

	videoID, err := extractYouTubeID(videoURL)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("Extracted Video ID:", videoID)
	ytClient := youtube.Client{}
	filePath := downloadVideo(videoID, &ytClient)

	apiKey := os.Getenv("AA_KEY")
	ctx := context.Background()
	aaClient := assemblyai.NewClient(apiKey)

	f, err := os.Open(filePath)
	if err != nil {
		log.Fatal("Couldn't open video file:", err)
	}
	defer f.Close()

	var generatedText string
	log.Println("Taking a moment to Transcribe...")
	ttext, err := os.ReadFile(filePath + ".txt")
	if err != nil {
		if os.IsNotExist(err) {
			transcript, err := aaClient.Transcripts.TranscribeFromReader(ctx, f, nil)
			if err != nil {
				log.Fatal("Something bad happened:", err)
			}
			generatedText = *transcript.Text
			// Save transcript to file
			err = os.WriteFile(filePath+".txt", []byte(generatedText), 0644)
			if err != nil {
				log.Fatal("Something bad happened:", err)
			}

		}
	} else {
		generatedText = string(ttext)
	}

	var c chatRequest

	system := message{
		Role: System,
		Content: fmt.Sprint(`You are a helpful assistant. 
			You will help this user answer questions that your users have based on the transcript to a youtube video.
			You will begin a with very short synopsis of the video. It is important that your synopsis is short. However, when a user begins asking questions, feel free to elaborate in order to satisfy the user.
			Here is the transcript: `, generatedText),
	}

	c = chatRequest{
		Model:   "llama3",
		Message: []message{system},
	}
	generateResponse(c)

	for {
		prompt := readPrompt()

		if prompt == "/bye" {
			break
		}

		c.Message = append(c.Message, message{
			Role:    User,
			Content: prompt,
		})

		generateResponse(c)

	}

}
