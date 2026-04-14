# Guildstate Keys

This file documents the currently known guildstate keys used by the bot.

Guildstate supports two key/value stores per guild:

- string settings (`MiscSettingsString`)
- boolean flags (`MiscSettingsFlag`)

## String Settings

| Key                  | Scope                  | Description                                                                                                 | Used By                                                        |
| -------------------- | ---------------------- | ----------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------- |
| `home_guild`         | `DEFAULT` pseudo-guild | Sets the home guild ID for admin guildstate workflows and guild-scoped admin command registration behavior. | `src/boost/boost_admin.go`, `src/boost/admin_contract_list.go` |
| `admin_logs_channel` | Real guild ID          | Channel ID used as the destination for admin contract log output.                                           | `src/boost/boost_menu.go`                                      |

## Boolean Flag Settings

| Key                               | Scope         | Description                                                        | Used By                        |
| --------------------------------- | ------------- | ------------------------------------------------------------------ | ------------------------------ |
| `active-contracts-show-completed` | Real guild ID | When `true`, include completed contracts in active contract views. | `src/boost/contract_active.go` |

## Dynamic Keys

Guildstate also allows arbitrary keys through admin commands and helper methods:

- `SetGuildSettingString(guildID, key, value)`
- `SetGuildSettingFlag(guildID, key, value)`

That means additional keys may exist in persisted data even if they are not hardcoded in source.

## Notes

- The `DEFAULT` guild ID is used as a global/default namespace for shared bot settings.
- Clearing a string setting is done by writing an empty value.
