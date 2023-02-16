`baton-slack` is a connector for Slack built using the [Baton SDK](https://github.com/conductorone/baton-sdk). It communicates with the Slack API to sync data about workspaces, users, user groups and channels.

Check out [Baton](https://github.com/conductorone/baton) to learn more the project in general.

# Getting Started

## Prerequisites
1. Create a Slack app. You can follow [this](https://api.slack.com/authentication/basics) guide.
2. Set needed scopes for the app (Bot Token Scopes): 
  - channels:join
  - channels:read
  - groups:read
  - team:read
  - usergroups:read
  - users.profile:read
  - users:read
  - users:read.email
3. Install the app to your workspace.
4. Use Bot User OAuth Token as token in baton-slack
## brew

```
brew install conductorone/baton/baton conductorone/baton/baton-slack
baton-slack
baton resources
```

## docker

```
docker run --rm -v $(pwd):/out -e BATON_TOKEN=token ghcr.io/conductorone/baton-slack:latest -f "/out/sync.c1z"
docker run --rm -v $(pwd):/out ghcr.io/conductorone/baton:latest -f "/out/sync.c1z" resources
```

## source

```
go install github.com/conductorone/baton/cmd/baton@main
go install github.com/conductorone/baton-slack/cmd/baton-slack@main

BATON_TOKEN=token
baton resources
```

# Data Model

`baton-slack` will pull down information about the following Slack resources:
- Workspace
- Users
- User Groups
- Channels

By default, baton-slack will sync information about default channels of user groups. You can specify additional channels you would like to sync using the --channel-ids flag.

# Contributing, Support and Issues

We started Baton because we were tired of taking screenshots and manually building spreadsheets. We welcome contributions, and ideas, no matter how small -- our goal is to make identity and permissions sprawl less painful for everyone. If you have questions, problems, or ideas: Please open a Github Issue!

See [CONTRIBUTING.md](https://github.com/ConductorOne/baton/blob/main/CONTRIBUTING.md) for more details.

# `baton-slack` Command Line Usage

```
baton-slack

Usage:
  baton-slack [flags]
  baton-slack [command]

Available Commands:
  completion         Generate the autocompletion script for the specified shell
  help               Help about any command

Flags:
      --channel-ids []string                IDs of additional Slack channels to sync ($BATON_CHANNEL_IDS)
  -f, --file string                         The path to the c1z file to sync with ($BATON_FILE) (default "sync.c1z")
      --token string                        The Access Token used to connect to the Slack API. ($BATON_TOKEN)
  -h, --help                                help for baton-slack
      --log-format string                   The output format for logs: json, console ($BATON_LOG_FORMAT) (default "json")
      --log-level string                    The log level: debug, info, warn, error ($BATON_LOG_LEVEL) (default "info")
  -v, --version                             version for baton-slack

Use "baton-slack [command] --help" for more information about a command.

```