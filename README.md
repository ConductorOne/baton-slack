`baton-slack` is a connector for Slack built using the [Baton SDK](https://github.com/conductorone/baton-sdk). It communicates with the Slack API to sync data about workspaces, users, user groups, and channels.

Check out [Baton](https://github.com/conductorone/baton) to learn more the project in general.

# Getting Started

## Prerequisites

1. Create a Slack app. You can follow [this Slack quickstart guide](https://api.slack.com/authentication/basics).
2. Set needed Bot Token Scopes for the app: 
  - channels:join
  - channels:read
  - groups:read
  - team:read
  - usergroups:read
  - users.profile:read
  - users:read
  - users:read.email
3. Install the app to your workspace.
4. Use Bot User OAuth Token as token in `baton-slack`.

For the enterprise grid plan the same rules apply for creating an app. There is 
a difference in setting scopes, for applications that will be installed on 
organization level, User Token Scopes should be set as well as bot scopes. User 
Token is used for Admin API needed to sync additional resources in the enterprise.
Additional scopes for User Token are:
  - admin
  - admin.roles:read
  - admin.teams:read
  - admin.usergroups:read
  - admin.users:read

For provisioning you will need this scope.
  - admin.users:write 

Other difference is in the way the application is installed, on enterprise grid 
app should be installed on the Organization level and on all the Workspaces from 
which you want to sync the resources. The installation has to be done by Admin 
or Owner of an Enterprise Grid organization. More info with an example is 
available in the [Slack API Docs](https://api.slack.com/methods/admin.teams.list#markdown).
To work with Enterprise Grid APIs use User OAuth Token passed as 
`--enterprise-token` along with the Bot User OAuth Token passed via `--token` flag.
 

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

`baton-slack` pulls down information about the following Slack resources:
- Workspaces
- Users
- User Groups
- Channels
- Workspace roles

Enterprise grid additional resources:
- Enterprise roles

With SSO configured (enterprise grid):
- IDP groups

If you have SSO configured for your enterprise grid organization you can also 
sync IDP groups and provision them. Just pass the `--sso-enabled=true` flag.

# Contributing, Support, and Issues

We started Baton because we were tired of taking screenshots and manually 
building spreadsheets. We welcome contributions, and ideas, no matter how 
small&mdash;our goal is to make identity and permissions sprawl less painful for 
everyone. If you have questions, problems, or ideas: Please open a GitHub Issue!

See [CONTRIBUTING.md](https://github.com/ConductorOne/baton/blob/main/CONTRIBUTING.md) for more details.

# `baton-slack` Command Line Usage

```
baton-slack

Usage:
  baton-slack [flags]
  baton-slack [command]

Available Commands:
  capabilities       Get connector capabilities
  completion         Generate the autocompletion script for the specified shell
  help               Help about any command

Flags:
      --client-id string          The client ID used to authenticate with ConductorOne ($BATON_CLIENT_ID)
      --client-secret string      The client secret used to authenticate with ConductorOne ($BATON_CLIENT_SECRET)
      --enterprise-token string   The Slack user oauth token used to connect to the Slack Enterprise Grid Admin API ($BATON_ENTERPRISE_TOKEN)
  -f, --file string               The path to the c1z file to sync with ($BATON_FILE) (default "sync.c1z")
  -h, --help                      help for baton-slack
      --log-format string         The output format for logs: json, console ($BATON_LOG_FORMAT) (default "json")
      --log-level string          The log level: debug, info, warn, error ($BATON_LOG_LEVEL) (default "info")
  -p, --provisioning              This must be set in order for provisioning actions to be enabled ($BATON_PROVISIONING)
      --skip-full-sync            This must be set to skip a full sync ($BATON_SKIP_FULL_SYNC)
      --sso-enabled               Flag indicating that the SSO has been configured for Enterprise Grid Organization. Enables usage of SCIM API ($BATON_SSO_ENABLED)
      --ticketing                 This must be set to enable ticketing support ($BATON_TICKETING)
      --token string              required: The Slack bot user oauth token used to connect to the Slack API ($BATON_TOKEN)
  -v, --version                   version for baton-slack

Use "baton-slack [command] --help" for more information about a command.
```
