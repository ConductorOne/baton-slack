package main

import (
	cfg "github.com/conductorone/baton-slack/pkg/config"
	"github.com/conductorone/baton-sdk/pkg/config"
)

func main() {
	config.Generate("slack", cfg.Configuration)
}
