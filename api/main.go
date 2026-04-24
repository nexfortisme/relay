package main

import (
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
)

func main() {
	fmt.Println("Hello, World!")

	c := cron.New(cron.WithSeconds())
	c.AddFunc("*/2 * * * * *", func() {
		fmt.Println("Heartbeat", time.Now().Format(time.RFC3339))
		heartbeat()
	})

	fmt.Println("Starting cron")
	c.Start()
	defer c.Stop()

	select {}
}

func heartbeat() {
	fmt.Println("Heartbeat")
}
