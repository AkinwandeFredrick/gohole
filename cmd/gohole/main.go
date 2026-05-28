package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
)

const version = "dev"

func main() {
	var (
		configPath = flag.String("config", "config.yaml", "Path to configuration file")
		showVersion = flag.Bool("version", false, "Show version information")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("gohole version %s\n", version)
		os.Exit(0)
	}

	log.Println("Starting gohole - Pi-hole style DNS sinkhole")
	log.Printf("Config path: %s\n", *configPath)

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// TODO: Add DNS server initialization
	// TODO: Load configuration from configPath
	// TODO: Initialize web dashboard

	log.Println("gohole is running. Press Ctrl+C to stop.")

	// Wait for shutdown signal
	<-sigChan
	log.Println("Shutting down gohole...")
}
