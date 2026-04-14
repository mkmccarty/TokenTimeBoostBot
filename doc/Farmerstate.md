# Farmerstate Keys

This file documents the currently known farmerstate keys used by the bot.

Farmerstate stores sticky per-user values in:
- string settings (`MiscSettingsString`)
- boolean flags (`MiscSettingsFlag`)

## String Settings

| Key                          | Scope    | Description                                                               | Used By                                                                                                                                                                                                                                     |
| ---------------------------- | -------- | ------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `encrypted_ei_id`            | Per user | User's encrypted Egg, Inc ID used for API-backed features.                | `src/boost/register.go`, `src/boost/eggidmodal.go`, `src/boost/contract_report.go`, `src/boost/replay.go`, `src/boost/leaderboard.go`, `src/boost/estimate_scores.go`, `src/boost/lobby.go`, `src/boost/stones.go`, `src/boost/teamwork.go` |
| `ei_ign`                     | Per user | Cached Egg, Inc in-game name (IGN).                                       | `src/boost/register.go`, `src/boost/contract_report.go`, `src/boost/replay.go`, `src/boost/boost_admin.go`, `src/boost/virtue.go`                                                                                                           |
| `TE`                         | Per user | Cached Truth Egg count used in ordering/scoring displays.                 | `src/boost/register.go`, `src/boost/update_farmer.go`, `src/boost/boost.go`, `src/boost/virtue.go`                                                                                                                                          |
| `defl`                       | Per user | Preferred Tachyon Deflector artifact tier/rarity (for coop calculations). | `src/boost/artifacts.go`, `src/boost/boost.go`, `src/ei/ei_artifacts.go`                                                                                                                                                                    |
| `metr`                       | Per user | Preferred Quantum Metronome artifact tier/rarity.                         | `src/boost/artifacts.go`, `src/boost/boost.go`, `src/ei/ei_artifacts.go`                                                                                                                                                                    |
| `comp`                       | Per user | Preferred Interstellar Compass artifact tier/rarity.                      | `src/boost/artifacts.go`, `src/boost/boost.go`, `src/ei/ei_artifacts.go`                                                                                                                                                                    |
| `guss`                       | Per user | Preferred Ornate Gusset artifact tier/rarity.                             | `src/boost/artifacts.go`, `src/boost/boost.go`, `src/ei/ei_artifacts.go`                                                                                                                                                                    |
| `collegg`                    | Per user | Selected Colleggtibles list (comma-separated egg names).                  | `src/boost/artifacts.go`, `src/boost/register.go`, `src/boost/boost.go`                                                                                                                                                                     |
| `EggIncRawName`              | Per user | Saved raw Egg, Inc name text used by teamwork/report commands.            | `src/farmerstate/farmerstate.go`, `src/boost/teamwork.go`                                                                                                                                                                                   |
| `AltController`              | Per user | Discord user ID of the controlling/main account for an alt account.       | `src/boost/boost_change.go`, `src/boost/boost.go`                                                                                                                                                                                           |
| `timer`                      | Per user | Sticky duration used by timer command defaults.                           | `src/boost/timer.go`                                                                                                                                                                                                                        |
| `scoreCalcParams`            | Per user | Serialized score explorer settings for load/save actions.                 | `src/boost/score_explorer.go`                                                                                                                                                                                                               |
| `virtueCompactMode`          | Per user | Stores compact-mode preference for virtue views (`"true"`/`"false"`).     | `src/boost/virtue.go`                                                                                                                                                                                                                       |
| `huntMinimumDrops`           | Per user | Menno hunt minimum drops preference.                                      | `src/menno/command.go`                                                                                                                                                                                                                      |
| `huntItemDuration`           | Per user | Menno hunt item duration preference.                                      | `src/menno/command.go`                                                                                                                                                                                                                      |
| `allow_coop_status`          | Per user | Timestamp (RFC3339) of Coop Status permission grant.                      | `src/boost/coop_status_permission.go`                                                                                                                                                                                                       |
| `allow_coop_status_span`     | Per user | Coop Status permission window (`24h` or `7d`).                            | `src/boost/coop_status_permission.go`                                                                                                                                                                                                       |
| `allow_leaderboard_api`      | Per user | Timestamp (RFC3339) of leaderboard permission grant.                      | `src/boost/leaderboard_permission.go`                                                                                                                                                                                                       |
| `allow_leaderboard_api_span` | Per user | Leaderboard permission window (`24h` or `forever`).                       | `src/boost/leaderboard_permission.go`                                                                                                                                                                                                       |

## Boolean Flag Settings

| Key                     | Scope    | Description                                                                | Used By                                                |
| ----------------------- | -------- | -------------------------------------------------------------------------- | ------------------------------------------------------ |
| `ultra`                 | Per user | Sticky preference to include ultra event output in event-related commands. | `src/events/event.go`, `src/events/launch_slashcmd.go` |
| `event-private`         | Per user | Sticky preference for private (ephemeral) event helper replies.            | `src/events/event.go`                                  |
| `calc-details`          | Per user | Sticky preference to include detailed output in token value calculations.  | `src/boost/boost_calctval.go`                          |
| `SuppressContractPings` | Per user | Whether contract-role pings should be suppressed for this user.            | `src/boost/boost_slashcmd.go`                          |
| `stone-details`         | Per user | Sticky preference to show detailed stones report output.                   | `src/boost/stones.go`                                  |
| `stone-tiled`           | Per user | Sticky preference to render stones output in tiled mode.                   | `src/boost/stones.go`                                  |

## Dynamic Keys

Farmerstate supports arbitrary keys through helper methods:
- `SetMiscSettingString(userID, key, value)`
- `SetMiscSettingFlag(userID, key, value)`

Dynamic writes currently observed:
- Artifact keys are written dynamically from inventory sync (`defl`, `metr`, `comp`, `guss`) in `src/boost/register.go` via `ei.GetBestCoopArtifactsFromInventory`.
- Artifact UI also writes by dynamic command key (`defl`, `metr`, `comp`, `guss`, `collegg`) in `src/boost/artifacts.go`.

## Test-only Keys

The key `guildID` appears in `src/farmerstate/farmerstate_test.go` for test setup; it is not currently used by production command flows.

## Notes

- Settings are per user (keyed by Discord user ID), not per guild.
- Clearing a string setting is done by writing an empty value.
- Writes are skipped when `DataPrivacy` is enabled for the user.
