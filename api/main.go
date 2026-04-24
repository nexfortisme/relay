package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/robfig/cron/v3"
)

// For Reading in from the .env file
func init() {

	// TODO - Will need to make this dynamic based on the directory it is running from
	// Since this will be running from the api directory, we need to go up one level to get to the root directory
	envFile := filepath.Join("../.env")
	err := godotenv.Overload(envFile)
	if err != nil {
		fmt.Printf("Error loading .env file: %v", err)
	}

	// -- Variables Set --
	fmt.Println("\nSecrets Set:")
	fmt.Println("LLM_MODEL Set:", os.Getenv("LLM_MODEL") != "")
	fmt.Println("LLM_URL Set:", os.Getenv("LLM_URL") != "")
}

func main() {
	fmt.Println("Hello, World!")

	c := cron.New(cron.WithSeconds())
	c.AddFunc("*/2 * * * * *", func() {
		heartbeat()
	})

	c.Start()
	defer c.Stop()

	select {}
}

func heartbeat() {
	fmt.Println("Heartbeat")
}
