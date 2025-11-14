package main

import (
	"github.com/conductorone/baton-sdk/pkg/config"
	cfg "github.com/conductorone/baton-slack/pkg/config"
)

func main() {
	config.Generate("slack", cfg.Configuration)
}
