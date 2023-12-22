# TokenTimeBoostBot

Boost Order Management Discord Bot

Allow for easy mobile creation and execution of a signup list and boost list for
the game Egg Inc.

The bot does not query information from the Egg Inc servers.



## Basic Setup
Create a discord application here: https://discord.com/developers/applications
Take the Application ID and Secret and save those to configure the bot in .config.json.
If you wish to restrict the bot to a specific server then add your Guild ID to **DiscordGuildID**

    "DiscordToken": "APP_SECRET"
    "DiscordAppID": "APP_ID"
    "DiscordGuildID": "DISCORD_GUILD_ID"

Install your bot into your discord server with this URL:
https://discord.com/api/oauth2/authorize?client_id=$(BOT_APP_ID)&permissions=466004470848&scope=bot%20applications.commands



## Slash commands ##

### Create Contract Coop ###

/contract contract-id coop-id coop-size

This will display a Sign-up List and a message with reactions for
players to sign up.
When the Sign-up List reaches the coop-size it will automatically
start the contract
The reactions are farmer, bell and dice.
Select farmer and/or bell to sign up on the list
Select the dice as a vote to randomize the Boost List.  
Normally the Boost List will will run in Sign-up Order.  
The vote needs a 2/3 super-majority before electing the random order.

### Start Contract ###

/start

This will change the Sign-up List to the Boost List. If there is a
order preference it will apply before the Boost List is displayed.
The first farmer on the list is presented with a boost token indicating
that they are the current booster.
The channel receives a message mentioning who's turn it is.
Farmers that reacted with a 🔔 will receive a DM about this.

### Boost ###

/boost

The Farmer who's turn it is to receive tokens uses this to indicate that they
are boosting.  
Contract Farmers may vote to indicate an AFK player has enough tokens to boost by
selecting the 🚀 icon.  Two votes will elect a successful boost.

### Skip Current Booster ###

/skip

Move current booster to last in the Boost List

### Move Farmer to Last ###

/last

Move yourself to last in the Boost List

### Prune Farmer ###

/prune

Remove a Farmer from the Sign-up or Boost List.
This is useful if a Farmer reacted to the Sign-up message and didn't join
the contract within the game.

### Join Farmer to Contract ###

/join farmer-mention

Add a farmer to the contract within that channel. The players are added
as Farmers without 🔔 DM notifications.

### Swap Current and Next Token Player ###

/swap 
command to swap yourself when currently boosting to next

### Priority Request ###

/priority to allow someone to signal they need to go early
Player has indicated that they wish to boost early for
Real Life reasons


