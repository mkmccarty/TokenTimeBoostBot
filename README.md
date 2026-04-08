[![Development](https://github.com/mkmccarty/TokenTimeBoostBot/actions/workflows/development.yml/badge.svg)](https://github.com/mkmccarty/TokenTimeBoostBot/actions/workflows/development.yml)

# Boost Bot

Boost Order Management Discord Bot

Allow for easy mobile creation and execution of a sign-up list and boost list for the game Egg Inc.
* There is a limitation of one contract per channel or thread.

## Basic Setup

Create a discord application here: <https://discord.com/developers/applications>

Take the Application ID and Secret and save those to configure the bot in .config.json.
If you wish to restrict the bot to a specific server then add your Guild ID to **DiscordGuildID**

```json
.config.json:
{
 "DiscordToken": "APP_SECRET",
 "DiscordAppID": "APP_ID",
 "DiscordGuildID": "DISCORD_GUILD_ID"
}
```

Install your bot into your discord server with this URL:
<https://discord.com/api/oauth2/authorize?client_id=$(BOT_APP_ID)&permissions=466004470848&scope=bot%20applications.commands>

## Slash commands

### Contract and Boosting

- `/contract` - Create a contract signup/boost workflow in the current channel.
- `/join-contract` - Add a farmer or guest to an existing contract.
- `/boost` - Mark the current booster as boosting.
- `/skip` - Move the current booster to the end of the boost order.
- `/unboost` - Mark a farmer as unboosted.
- `/prune` - Remove a farmer from the signup/boost list.
- `/change` - Update contract settings and boost order options.
- `/update` - Refresh contract data/status for the current contract.
- `/change-one-booster` - Change one booster entry in the running contract.
- `/change-start` - Update planned contract start timing.
- `/change-speedrun-sink` - Change speedrun sink settings.
- `/contract-settings` - Show configurable settings for the current contract.
- `/toggle-contract-pings` - Toggle sticky contract ping behavior.
- `/bump` - Redraw the boost list timeline.

### Reporting and Analysis

- `/contract-report` - Generate a report for a contract.
- `/score-explorer` - Open score explorer tools for a contract.
- `/teamwork` - Run teamwork evaluation.
- `/speedrun` - Run speedrun-related calculations/tools.
- `/estimate-contract-time` - Estimate contract completion time.
- `/cs-estimate` - Run CS estimate tools.
- `/coopeta` - Estimate coop completion from current rate/time.
- `/predictions` - Show prediction tools/pages.
- `/leaderboard` - Show leaderboard pages/data.
- `/stones` - Show stones tools/pages.
- `/timer` - Start/manage bot timer tools.

### Utility

- `/artifact` - Show artifact helper tools.
- `/link-alternate` - Link alternate account/player entries.
- `/calc-contract-tval` - Calculate contract T-value.
- `/coop-tval` - Calculate coop T-value.
- `/rename-thread` - Rename the current contract thread.
- `/seteggincname` - Set or update a player's Egg, Inc. in-game name.
- `/remove-dm-message` - Remove a DM tracking message.
- `/help` - Show bot help.
- `/privacy` - Show privacy information.

### Sinks and Volunteering

- `/volunteer-sink` - Volunteer as sink for speedrun flow.
- `/voluntell-sink` - Tell/assign sink volunteers.

### Global/DM Commands

- `/token` - Token tracking command set.
- `/token-edit` - Edit tracked token records.
- `/token-edit-track` - Edit token tracking in DM/global context.
- `/register` - Register player/profile information.
- `/virtue` - Virtue command/tools.
- `/rerun-eval` - Re-run evaluation for a contract.
- `/hunt` - Menno hunt helper command.
- `/launch-helper` - Launch helper command for event tooling.
- `/events` - Event helper commands.

### Optional Command

- `/fun` - Fun command set (only enabled when `NO_FUN` feature flag is not set).

### Admin Commands

- `/admin-contract-list` - List all running contracts.
- `/admin-contract-finish` - Mark a contract as finished by hash.
- `/admin-reload-contracts` - Force reload Egg, Inc. contract data.
- `/admin-get-contract-data` - Retrieve raw contract/co-op JSON data.
- `/list-roles` - Show role usage for a contract.
- `/admin-guildstate` - Guildstate admin command with guild override.
- `/admin-members` - Show admin member details.
- `/active-contracts` - Show active contracts overview.
- `/admin-set-guild-setting` - Set a guild setting.
- `/admin-get-guild-settings` - Get guild settings.
- `/status-message` - Set the next bot status message.
