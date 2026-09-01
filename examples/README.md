# Basic Configuration file

Place a configuration file in the same directory as the bot. When launched the bot will read this file to get the configuration settings.

```json
.config.json:
{
    "DiscordToken": "",
    "DiscordAppID": "",
    "DevBotAppID": "",
    "DiscordGuildID": "",
    "GoogleAPIKey": "",
    "AdminUserId": "",
    "AdminUsers": [],
    "EIUserId": "",
    "EIUserIdBasic": "",
    "FeatureFlags": [],
    "EventsURL": "https://wasmegg-carpet.netlify.app/events/",
    "RocketsURL": "https://wasmegg-carpet.netlify.app/rockets-tracker/"
}
```

## DiscodToken

This is the secret key for the bot. You can get this from the Discord Developer Portal.
Create a discord application here: <https://discord.com/developers/applications>

## DiscordAppID

This is the application ID for the bot. You can get this from the Discord Developer Portal.

## DevBotAppID

If you're using a development bot you need to specify the application ID for the development bot.
This allows the bot to select the correct set of emojii for the bot. You'll need to create a separate application in the Discord Developer Portal for the development bot and use the DiscordToken from a
development bot.

## DiscordGuildID

This is the server ID for the server you want to restrict the bot to.  When this is set to something then the commands will only be available on that server. To use commands via DM you need to leave this empty. This is an optional setting.


## GoogleAPIKey

This is the API key for the Google API. This is optional.
Used with the /fun commands to generate text messages

## AdminUserId

This is the user ID for the bot admin. This user can use all bot commands regardless of server permissions.
Other discord server users that have the PermissionAdministrator permission can also function as bot admin.

## EIUserId

This is the game Egg, Inc. user ID (EID) used for all other bot requests, such as retrieving contracts, periodicals, and leaderboards. To ensure the bot receives Ultra events and contracts in a timely manner, this account needs to be an active game user with an Ultra subscription.

For iOS you could create a second AppleID and with that create a new GameCenter account to use for this purpose. With your primary AppleID you can sign out of Game Center and then sign in with the GameCenter ID from the new AppleID. Once in the game you can Restore Purchases to get the Ultra subscription added to this new EI User ID. You don't need to do anything else with this account.

## EIUserIdBasic

This is a special bot user ID requested directly from AuxBrain that is used specifically for all `coop_status_bot` requests. This ID does not need an Ultra subscription.

> [!WARNING]
> Do not use a normal player's user ID for `EIUserIdBasic` / `coop_status_bot` requests. Using a personal user ID for these requests can result in lost tokens and lost chicken runs, and will limit lookups only to contracts that specific user is currently in.

## AdminUsers

If want to allow other Discord users to have admin permissions you can add their user IDs to this list.

## FeatureFlags

This list of strings are flags that can be used to enable or disable features in the bot.

### NO_FUN

This will remove the fun commands from the bot.

## EventsURL

This is the URL that will be displayed with the /events command. This is optional.

## RocketsURL

This is the URL that will be displayed with the /launch-helper command. This is optional.
