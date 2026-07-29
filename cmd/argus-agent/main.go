// argus-agent is a deliberately small, outbound-only private-agent process.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"argus/internal/agent"
)

var version = "dev"

func main() {
	controlURL := flag.String("control-url", os.Getenv("ARGUS_AGENT_CONTROL_URL"), "Argus HTTPS base URL")
	token := flag.String("token", os.Getenv("ARGUS_AGENT_TOKEN"), "private-agent enrollment token")
	configPublicKey := flag.String("config-public-key", os.Getenv("ARGUS_AGENT_CONFIG_PUBLIC_KEY"), "base64url Ed25519 public key for signed control-plane configuration")
	interval := flag.Duration("heartbeat-interval", 60*time.Second, "outbound heartbeat interval (15s to 24h)")
	flag.Parse()
	if *interval < 15*time.Second || *interval > 24*time.Hour {
		log.Fatal("heartbeat-interval must be between 15s and 24h")
	}
	client, err := agent.NewClient(agent.Config{ControlURL: *controlURL, Token: *token, Version: version, ConfigPublicKey: *configPublicKey})
	if err != nil {
		log.Fatalf("configure agent: %v", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if *configPublicKey != "" {
		if signed, configErr := client.FetchConfiguration(ctx); configErr != nil {
			log.Fatalf("verify agent configuration: %v", configErr)
		} else {
			*interval = time.Duration(signed.HeartbeatIntervalSeconds) * time.Second
		}
	}
	for {
		if err := client.Heartbeat(ctx); err != nil {
			// Credentials and endpoint query data are never logged.
			log.Printf("agent heartbeat failed: %v", err)
		} else {
			log.Printf("agent heartbeat accepted (version %s)", version)
		}
		timer := time.NewTimer(*interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
