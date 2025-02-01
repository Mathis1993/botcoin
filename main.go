package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"botcoin/config"
	"botcoin/trading"
)

func main() {
	configPath := flag.String("config", "config.json", "path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create and start the trading bot
	bot, err := trading.NewBot(cfg)
	if err != nil {
		log.Fatalf("Failed to create trading bot: %v", err)
	}

	if err := bot.Start(); err != nil {
		log.Fatalf("Failed to start trading bot: %v", err)
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("Shutting down...")
	
	if err := bot.Stop(); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}
}
