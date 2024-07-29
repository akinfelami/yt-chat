package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
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

func main() {

	fmt.Println("++++++++++++++++++++==++++++++++++++++++++++++++++++++++++")
	fmt.Println("Welcome to Ollama Chat!")

	var c chatRequest

	system := message{
		Role:    System,
		Content: "You are a helpful assistant. You will help this user answer questions. You are to be very helpful so if the user gives you an unfinished thought you will help them complete it, making sure to articulate yourself and your thought process.",
	}

	c = chatRequest{
		Model:   "llama3",
		Message: []message{system},
	}

	for {
		prompt := readPrompt()

		if prompt == "/bye" {
			break
		}

		c.Message = append(c.Message, message{
			Role:    User,
			Content: prompt,
		})

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

}
