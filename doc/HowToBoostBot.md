# How To Boost Bot

The Boost Bot helps contract coordinators and farmers with the running of a boost list. Farmers can add themselves, indicate tokens are being passed and mark as boosted.

## Contract Coordinator

A contract coordinator will start the contract with **/contract** and supply a contract-id and coop-id.
The coordinator will see a red *Delete Contract* message to use to delete the contract and start over.
Everyone will see a set of buttons to join, leave, set token preference and a **Start Boost List** button.

There is a limitation of a single running contract per channel or thread as messages to the bot use guild and channel id's to find the contract. Try to make sure that any running contracts are marked as completed by boosting everyone or using the 🏁 reaction.

To change from the sign-up list to the actual boost list the coordinator needs to press the green **Start Boost List** button. The sign-up list is intended to gather farmers for a contract before starting a game lobby. Once you have enough of a group to start your boost list then hit the button.

Start the contract with **/contract** and fill in the parameters contract-id coop-id coop-size boost-order and ping-role. The boost-order defaults to sign-up order. The ping-role defaults to `@here`, it can be changed with */change*
Use */join*, */prune*, */unboost* and */change* to help organize the contract.

The contract coordinator and some server admin's are able to react to the **Start Boost List** button and Boost-list reactions for :rocket: and 🔃 to help keep contracts moving.

## Users of Boost Bot

When the sign-up list is created the buttons for that are pinned for quick future access. When the the sign-up or boost list has a change to show a different booster, the updated list will be posted as a new message, and the previous one deleted. This keeps the list moving with the channel or thread's timeline.

From the pinned signup buttons, **Join** the contract and set the number of boost tokens you like to use, these default to 8 and anything set will persist for future contracts. The 5/6/8 set the number of tokens wanted with +/- adjusting that amount up or down so you get to any number.

When the boost list is shown, one farmer is the current booster with a display with the number of tokens wanted and several reaction icons. Select :token: each time you send a token to the current booster. If you are the current booster and receive a token on your own through ads or other means use the token reaction on yourself so everyone is aware how many tokens you still need.

When the current booster has enough tokens they'll select the :rocket: to advance the boost list. Sometimes RL takes priority and you may not be present indicate you have enough tokens to boost with, two :rocket: reactions by others will also trigger a boost and select the next booster. A single reaction by the coordinator would also trigger a boost.

If you want to move yourself to the bottom of the boost list then ⤵️ is your huckleberry. If you want to boost next then add the :toilet: reaction. For other boost order requests ask the coordinator.

## Other commands
/boost - Use this if you've boosted out of order.

/seteggicname - Use this to set your Egg, Inc game name.