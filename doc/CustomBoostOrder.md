# Custom Boost Orders

Boost Bot supports **Custom Boost Orders**, allowing contract coordinators to create multi-tier sorting rules using a concise comparison language. Custom orders can be tested, previewed via a rendered image table, saved as personal presets, published globally across servers, or applied directly to a live contract.

---

## 1. Defining a Custom Boost Order (`/custom-boost-order`)

Run the `/custom-boost-order` command:

* **Craft / Preview / Create**:
```text
/custom-boost-order craft [order]
```
* **Delete a Saved Order**:
```text
/custom-boost-order delete <order>
```
Displays the custom order rules (without image) with buttons to **DISMISS** (removes buttons) or **DELETE** (deletes the preset and confirms).
* **Help & Documentation**:
```text
/custom-boost-order help
```
Displays an ephemeral response with this guide (`CustomBoostOrder.md`) attached directly in Discord.

### Autocomplete Options
When running `/custom-boost-order craft`, the `order` parameter autocomplete provides:
* `<NEW>`: Create a new custom boost order from scratch.
* `[User] <name>`: Load a personal preset previously saved to your profile.
* `[Global] <name>`: Load a globally published preset accessible across all servers.

When running `/custom-boost-order delete`, autocomplete provides all saved presets (`[User] <name>` and `[Global] <name>`).

---

## 2. The 4-Line Criteria Modal

Selecting `<NEW>` or clicking **MODIFY** opens the **Custom Boost Order Criteria** modal with 4 rows:

| Row | Role | Required | Example |
| :--- | :--- | :---: | :--- |
| **Level 1** | Primary sorting rule | Yes | `<IHR[6%]` |
| **Level 2** | Secondary tiebreaker | No | `ELR` |
| **Level 3** | Tertiary tiebreaker | No | *(blank)* |
| **Level 4** | Quaternary tiebreaker | No | *(blank)* |

> [!NOTE]
> Higher levels take absolute precedence. Lower levels are only evaluated when players are **tied** on all preceding levels. If all 4 levels result in a tie, sign-up order is used as the fallback.

> [!TIP]
> **Case Insensitive**: All rule inputs are completely case-insensitive. You can type criteria in uppercase, lowercase, or mixed case (e.g. `<ihr[6%]`, `elr`, `>tokens`, `soul_mirror_orange`, `Artifact Score`, or `if role == main then ihr else te`).

---

## 3. Understanding `<` and `>` (Sorting Direction)

The symbols `<` and `>` are **direction prefixes** placed at the start of a condition. They are **not** HTML/XML closing tags.

* **`<` Prefix: Highest First (High to Low)**
  * Boosters with the highest numerical value or highest tier boost earliest.
  * In Egg, Inc. contracts, stronger stats almost always boost first. Therefore, **Highest First is the default** for stats even if you omit `<`.
  * *Examples:* `<IHR` (or `IHR`), `<TE`, `<ELR`, `<DEFL_EFFORT[50]`
* **`>` Prefix: Lowest First (Low to High)**
  * Boosters with the lowest numerical value boost earliest.
  * Most commonly used for **Tokens Wanted** (`>TOKENS`), so farmers who only need 4 tokens boost before farmers asking for 6 or 8 tokens.
  * Can also be used to invert any stat (e.g. `>IHR` to boost lowest IHR first in reverse runs).
  * *Examples:* `>TOKENS`, `>IHR`

> [!TIP]
> **No trailing `>`!** Write `<IHR[6%]`, not `<IHR[6%]>`. While the parser is forgiving and strips outer `<...>` if entered, `<` is simply a prefix indicating "highest to lowest".

---

## 4. Understanding Fuzzy Banding (e.g. `<IHR[6%]`)

### The Problem Without Fuzzy Banding
Without fuzzy banding, stats are compared down to tiny decimal values:
* Player A: `10.005B` IHR
* Player B: `10.000B` IHR
* Player A boosts first, even though the 0.05% difference is negligible in practice.

### How `[6%]` Solves This
Adding a bracketed percentage like `[6%]` creates a **fuzzy tolerance window**:
* Any players whose values are within $6\%$ of each other are considered **tied** on this level.
* Because they tie, the bot advances to the **next tiebreaker level** to determine who boosts first.

### Step-by-Step Example
Suppose your rule configuration is:
* **Level 1**: `<IHR[6%]` (Highest IHR first, with a 6% tie window)
* **Level 2**: `>TOKENS` (Fewer tokens wanted boosts earlier)

Consider three players in the contract:
| Player | IHR | Tokens Wanted |
| :--- | :--- | :---: |
| **Alice** | `10.2B` | 8 |
| **Bob** | `10.0B` | 4 |
| **Charlie** | `7.5B` | 4 |

**How the Bot Evaluates Them:**
1. **Level 1 Evaluation (`<IHR[6%]`)**:
   * Alice (`10.2B`) and Bob (`10.0B`) differ by only $2\%$, which is well within the $6\%$ window. They are considered **tied** on Level 1.
   * Charlie (`7.5B`) is $>6\%$ below Alice and Bob, so Charlie is placed behind them.
2. **Level 2 Tiebreaker (`>TOKENS`)**:
   * To break the tie between Alice and Bob, the bot looks at tokens requested.
   * Bob wants `4` tokens, while Alice wants `8` tokens.
   * Lowest tokens wins: **Bob is placed ahead of Alice**.
3. **Final Order**:
   1. **Bob** (10.0B IHR, 4 tokens)
   2. **Alice** (10.2B IHR, 8 tokens)
   3. **Charlie** (7.5B IHR, 4 tokens)

---

## 5. Balanced Deflector Effort (`DEFL_EFFORT[N]`)

The `DEFL_EFFORT[N]` condition allows players who have invested heavily into crafting a T4 Deflector to compete on equal footing with players who already own a T4L Deflector:

$$\text{Effort} = \begin{cases} N & \text{if booster owns a T4L Deflector} \\ \min(N, \text{craft\_attempts}) & \text{if booster does not own a T4L Deflector} \end{cases}$$

### Example (`<DEFL_EFFORT[50]`):
* **Owner of T4L Deflector**: Effort = **50**.
* **Player with 70 crafts (no T4L)**: Effort = $\min(50, 70) =$ **50** (equivalent to T4L).
* **Player with 50 crafts (no T4L)**: Effort = $\min(50, 50) =$ **50** (equivalent to T4L).
* **Player with 35 crafts (no T4L)**: Effort = **35**.

Because the T4L owner and the 70-craft & 50-craft players all tie at 50 effort, their order is determined by the **Level 2** rule (e.g. `<IHR[6%]` or `>TOKENS`). The 35-craft player boosts after them.

---

## 6. Roles & Conditional Rules (`IF ROLE ... ELSE ...`)

In Boost Bot, players in a contract have one of two roles:
* **Main**: Primary player accounts (the default for all participants).
* **Helper**: Alternate/helper accounts (designated alts or accounts marked via `/boost-order-helpers`).

Coordinators manage helper designations using:
* `/boost-order-helpers set farmers:...`
* `/boost-order-helpers clear farmers:...`
* `/boost-order-helpers list`

> [!NOTE]
> Items like Ship in a Bottle (`SIAB`), Gusset (`GUSSET`), or Compass (`COMPASS`) are **not roles**—they are artifact items. To prioritize players who have or equip these items, use artifact criteria directly (e.g. `<SIAB`, `<GUSSET`, or `<COMPASS`).

### Standalone Role Sorting
You can use `ROLE` as a standalone criterion row:
* `<ROLE` (or `ROLE`, `MAIN`): **Mains first**, Helpers second.
* `>ROLE` (or `HELPER`): **Helpers first**, Mains second.

### Conditional Sorting (`IF ROLE ... ELSE ...`)
You can branch your sorting strategy based on whether a player is a Main or Helper:

```text
IF [ROLE ==] MAIN <THEN_CRITERIA> [ELSE <ELSE_CRITERIA>]
IF [ROLE ==] HELPER <THEN_CRITERIA> [ELSE <ELSE_CRITERIA>]
```

#### Syntax Examples:
* `IF MAIN <DEFL_EFFORT[50] ELSE >TOKENS`
  * Mains are placed first and sorted by Deflector Effort.
  * Helpers are placed second and sorted by Tokens Wanted (fewer tokens boost earlier).
* `IF HELPER >TOKENS ELSE <DEFL_EFFORT[50]`
  * Helpers are placed first and sorted by Tokens Wanted.
  * Mains are placed second and sorted by Deflector Effort.

### When `ELSE` is Missing (Sort Does Not Apply)
If `ELSE` is omitted, **the sort rule does not apply to non-matching boosters**:
* Matching boosters are sorted among themselves by the `THEN` rule and placed ahead of non-matching boosters.
* Non-matching boosters **tie on this level** and fall through to the subsequent tiebreaker rows (Level 2, Level 3, etc.).

#### Example:
* **Level 1**: `IF MAIN <DEFL_EFFORT[50]` *(no ELSE)*
* **Level 2**: `>TOKENS`

**How boosters are evaluated:**
1. All **Mains** are ranked by Deflector Effort on Level 1.
2. All **Helpers** do not match `MAIN` and have no `ELSE` rule, so Level 1 **does not apply** to them. All Helpers tie on Level 1.
3. Level 2 (`>TOKENS`) breaks the tie among the Helpers, sorting them by lowest tokens wanted.
4. Final Order: Mains (sorted by Defl Effort), followed by Helpers (sorted by Tokens Wanted).

---

## 7. Complete Criteria Reference

| Symbol | Meaning | Default Direction | Description & Usage |
| :--- | :--- | :---: | :--- |
| `IF MAIN ... ELSE ...` | Role Conditional | — | Branches sorting rule based on player's role (`MAIN` or `HELPER`). |
| `ROLE` / `<ROLE` | Role Priority | `<` (Mains first) | Sorts Mains first, Helpers second. `>ROLE` (or `HELPER`) reverses this (Helpers first). |
| `DEFL_EFFORT[N]` | Balanced Deflector Effort | `<` (Highest) | Balances T4L owners with non-T4L players having $\ge N$ crafts. Defaults to $N=50$. |
| `CRAFT_DEFL` | Deflector Craft Attempts | `<` (Highest) | Raw count of T4 Deflector craft attempts. |
| `IHR` | Boosting IHR | `<` (Highest) | Effective Internal Hatchery Rate including artifact set bonuses. |
| `IHR[X%]` | Fuzzy IHR | `<` (Highest) | Groups IHR within $X\%$ as a tie to fall through to the next level. |
| `ELR` | Egg Laying Rate | `<` (Highest) | Laying rate calculated from equipped artifacts and stones. |
| `TE` | Truth Eggs (PE) | `<` (Highest) | Total Prophecy / Truth Egg count. |
| `TE[X%]` | Fuzzy TE | `<` (Highest) | Groups TE counts within $X\%$ as a tie. |
| `TE[sqrt]` | Square-Root TE | `<` (Highest) | Compresses large TE differences so other tiebreakers play a larger role. |
| `TOKENS` | Tokens Requested | `>` (Lowest) | Tokens requested for boosting. Use `>TOKENS` for lowest-first. |
| `TVAL` | Token Value | `<` (Highest) | Historical Token Value metric. |
| `DEFL` | Deflector Quality | `<` (Highest) | Deflector rarity tier (T4L > T4E > T4R > T4C > T3 > ...). |
| `DEFL_SLOT` | Deflector Stone Slots | `<` (Highest) | Number of stone slots available on the equipped deflector. |
| `DELIV` | Delivery Rate | `<` (Highest) | Effective delivery rate: $\min(\text{Egg Laying Rate}, \text{Shipping Rate})$ based on delivery artifacts and colleggtibles. |
| `SIGNUP` | Sign-up Order | `<` (First-in) | Order in which players signed up in the contract thread. |
| `RANDOM` | Deterministic Random | — | Stable pseudo-random tiebreaker. |
| `T4L_ACTUATOR` / `<T4L_ACTUATOR` | Artifact Ownership (Boolean) | `<` (Has it) | Boolean check: players possessing $\ge 1$ of the item rank first; non-owners rank second. |
| `COUNT(T4L_ACTUATOR)` | Artifact Quantity | `<` (Highest) | Exact inventory count of a specific artifact tier & rarity. |
| `CRAFT(T4_ACTUATOR)` | Artifact Craft Count | `<` (Highest) | Total craft attempts for a given artifact tier (e.g. `CRAFT(T4_ACTUATOR)`, `T4_ACTUATOR_CRAFTS`). |

---

## 8. Artifact Counts & Crafts Syntax

You can target any artifact in Egg, Inc. by ownership status, inventory count, or crafting attempts:

### 1. Artifact Ownership (Boolean Has-It)
Specify the Tier (`T1`–`T4`) and optional Rarity (`C`, `R`, `E`, `L` or Common, Rare, Epic, Legendary):
* `T4L_ACTUATOR` or `<T4L_ACTUATOR`: Boolean check. Boosters who own $\ge 1$ T4 Legendary Actuators boost first (all owners tie on this level). Boosters with 0 boost second.
* `T4E_GUSSET`: Boosters who own a T4 Epic Gusset boost first.
* `T4_COMPASS`: Boosters who own any T4 Compass boost first.
* `HAS(T4L_ACTUATOR)`: Explicit wrapper syntax.

### 2. Artifact Inventory Quantity (`COUNT`)
Use `COUNT(...)` or `QTY(...)` to sort by the exact number of items owned:
* `COUNT(T4L_ACTUATOR)`: Boosters holding 2 T4L Actuators rank ahead of boosters holding 1, who rank ahead of boosters holding 0.
* `COUNT(T4E_GUSSET)`: Total count of T4 Epic Gussets.

### 3. Artifact Craft Attempts
Specify crafting attempts for any artifact tier (defaults to T4 if tier is omitted):
* `CRAFT(T4_ACTUATOR)` or `CRAFT[T4_ACTUATOR]`: Total number of T4 Actuators crafted.
* `CRAFT_T4_ACTUATOR` or `T4_ACTUATOR_CRAFTS`: Equivalent shorthand forms.
* `CRAFT(ACTUATOR)`: Defaults to T4 Actuator crafts.
* `CRAFT(DEFL)` / `CRAFT_DEFL`: Tachyon Deflector crafts.

### 3. Supported Artifact Names & Aliases
All 21 Egg, Inc. artifacts and their common abbreviations are supported:
* **Actuator**: `ACTUATOR`, `TITANIUM_ACTUATOR`
* **Deflector**: `DEFL`, `DEFLECTOR`, `TACHYON_DEFLECTOR`
* **Metronome**: `METR`, `METRONOME`, `QUANTUM_METRONOME`
* **Compass**: `COMP`, `COMPASS`, `INTERSTELLAR_COMPASS`
* **Gusset**: `GUSS`, `GUSSET`, `ORNATE_GUSSET`
* **Chalice**: `CHALICE`, `THE_CHALICE`
* **Book of Basan**: `BOB`, `BOOK`, `BOOK_OF_BASAN`
* **Feather**: `FEATHER`, `PHOENIX_FEATHER`
* **Ankh**: `ANKH`, `TUNGSTEN_ANKH`
* **Brooch**: `BROOCH`, `AURELIAN_BROOCH`
* **Rainstick**: `RAINSTICK`, `CARVED_RAINSTICK`
* **Puzzle Cube**: `CUBE`, `PUZZLE_CUBE`
* **Ship in a Bottle**: `SIAB`, `SHIP`, `SHIP_IN_A_BOTTLE`
* **Monocle**: `MONOCLE`, `DILITHIUM_MONOCLE`
* **Lens**: `LENS`, `MERCURYS_LENS`
* **Totem**: `TOTEM`, `LUNAR_TOTEM`
* **Medallion**: `MEDALLION`, `NEODYMIUM_MEDALLION`
* **Beak**: `BEAK`, `BEAK_OF_MIDAS`
* **Light of Eggendil**: `LIGHT`, `LIGHT_OF_EGGENDIL`
* **Necklace**: `NECKLACE`, `DEMETERS_NECKLACE`
* **Vial of Martian Dust**: `VIAL`, `VIAL_OF_MARTIAN_DUST`

---

## 9. 📊 Player Stats, Inventory & Boost Counts

You can sort directly by a wide variety of player account stats and owned boost item inventories retrieved from the Egg, Inc. backup:

| Metric Keyword | Description | Default Order | Preview Column |
| :--- | :--- | :---: | :---: |
| `ARTIFACT_SCORE`, `ART_SCORE` | Total artifact inventory score | Highest first | `Art Score` |
| `CRAFTING_XP`, `CRAFT_XP`, `CXP` | Crafting experience / level | Highest first | `Craft XP` |
| `BOOST(<raw_id>)` (e.g. `BOOST(soul_mirror_orange)`) | Count of specific owned boost item by its raw game ID | Highest first | Boost Name |
| `GE`, `GOLDEN_EGGS` | Current Golden Eggs balance (earned - spent) | Highest first | `GE` |
| `EB`, `EARNINGS_BONUS` | Earnings Bonus percentage | Highest first | `EB` |
| `SE`, `SOUL_EGGS` | Total Soul Eggs count | Highest first | `SE` |
| `CTE`, `CLOTHED_TRUTH_EGGS` | Maximum Clothed Truth Eggs | Highest first | `CTE` |
| `PRESTIGE`, `PRESTIGES` | Lifetime prestiges count | Highest first | `Prestiges` |
| `DRONE`, `DRONES` | Lifetime standard drone takedowns | Highest first | `Drones` |
| `ELITE_DRONE`, `ELITE_DRONES` | Lifetime elite drone takedowns | Highest first | `Elite Drones` |

Prefixing any of these with `>` sorts lowest first (e.g. `>EB` to boost lower EB players first).

### Raw Boost Item IDs Reference

When using `BOOST(<raw_id>)`, specify the internal game `boost_id`. The list below covers all standard Egg, Inc. boost items:

#### Soul Mirrors
| In-Game Boost Name | Raw Boost ID | Duration & Effect |
| :--- | :--- | :--- |
| **Soul Mirror (10m)** | `soul_mirror_blue` | 10 min, match highest co-op EB |
| **Soul Mirror (1h)** | `soul_mirror_purple` | 1 hour, match highest co-op EB |
| **Soul Mirror (1d)** | `soul_mirror_orange` | 1 day, match highest co-op EB |

#### Tachyon Prisms (Internal Hatchery Rate)
| In-Game Boost Name | Raw Boost ID | Duration & Effect |
| :--- | :--- | :--- |
| **Tachyon Prism** | `tachyon_prism_blue` | 10 min, 10x hatchery |
| **Large Tachyon Prism** | `tachyon_prism_blue_big` | 4 hours, 10x hatchery |
| **Powerful Tachyon Prism** | `tachyon_prism_purple` | 10 min, 100x hatchery |
| **Epic Tachyon Prism** | `tachyon_prism_purple_big` | 2 hours, 100x hatchery |
| **Legendary Tachyon Prism** | `tachyon_prism_orange` | 10 min, 1000x hatchery |
| **Supreme Tachyon Prism** | `tachyon_prism_orange_big` | 1 hour, 1000x hatchery |

#### Boost Beacons (All Boost Multiplier)
| In-Game Boost Name | Raw Boost ID | Duration & Effect |
| :--- | :--- | :--- |
| **Boost Beacon** | `boost_beacon_blue` | 30 min, 2x boost multiplier |
| **Large Boost Beacon** | `boost_beacon_blue_big` | 2 hours, 2x boost multiplier |
| **Epic Boost Beacon** | `boost_beacon_purple` | 10 min, 10x boost multiplier |
| **Legendary Boost Beacon** | `boost_beacon_orange` | 10 min, 50x boost multiplier |

#### Soul Beacons (Prestige Soul Egg Collection)
| In-Game Boost Name | Raw Boost ID | Duration & Effect |
| :--- | :--- | :--- |
| **Soul Beacon** | `soul_beacon_blue` | 20 min, 5x soul eggs |
| **Epic Soul Beacon** | `soul_beacon_purple` | 30 min, 50x soul eggs |
| **Legendary Soul Beacon** | `soul_beacon_orange` | 10 min, 500x soul eggs |

#### Jimbo's Best Bird Feed (Earnings Multiplier)
| In-Game Boost Name | Raw Boost ID | Duration & Effect |
| :--- | :--- | :--- |
| **Jimbo's Best Bird Feed (20m)** | `jimbos_blue` | 20 min, 3x earnings |
| **Jimbo's Best Bird Feed (2h)** | `jimbos_blue_big` | 2 hours, 3x earnings |
| **Jimbo's Best Bird Feed (2h 10x)** | `jimbos_purple` | 2 hours, 10x earnings |
| **Jimbo's Best Bird Feed (8h 10x)** | `jimbos_purple_big` | 8 hours, 10x earnings |
| **Jimbo's Best Bird Feed (10m 50x)** | `jimbos_orange` | 10 min, 50x earnings |
| **Jimbo's Best Bird Feed (1h 50x)** | `jimbos_orange_big` | 1 hour, 50x earnings |

#### Utilities & Cash
| In-Game Boost Name | Raw Boost ID | Effect |
| :--- | :--- | :--- |
| **Quantum Warming Bulb** | `dilithium_bulb` | 10 min infinite running chickens |
| **Money Printer** | `money_printer` | Instant cash payout |
| **Blank Check** | `blank_check` | Instant cash payout |

---

## 10. ⚙️ Preview & Evaluation Report

Submitting the criteria modal or loading a preset generates an interactive preview report containing:
1. **Tiebreaker Hierarchy**: Shows the exact condition configured for Levels 1–4.
2. **Table Image Preview**: Renders a PNG table displaying each booster's metrics.
   * **Focused Dynamic Columns**: The table always shows the position rank (`#`) and player name (`Player`), followed strictly by columns for the criteria actually included in the boost order (e.g. `T4L Actuator`, `T4 Actuator Crafts`, `Role`, `IHR`, `Tokens`, etc.). Unused metrics are omitted to keep the table clean and concise.
   * Must be run in an active contract channel to evaluate and display the contract's actual players.

### Action Buttons:
* **MODIFY**: Re-opens the 4-row criteria modal to tweak rules.
* **SAVE**: Opens a modal to enter a Name (pre-populated with a suggested name generated from your rules if not already named) and saves the preset to your personal profile.
* **SELECT**: *(Only in active contracts when saved/loaded)* Immediately applies the custom order to the contract boost list and updates contract boost order.

---

## 11. Applying Custom Orders in Contracts

1. **Via Dropdown (`cs_#order`)**:
   * In contract signup or `/contract-settings`, open the **Boosting Order** select menu.
   * All published global orders (`⚙️ [Global] <name>`) and personal presets (`👤 [User] <name>`) appear in the list.
2. **Via `/custom-boost-order craft`**:
   * Run `/custom-boost-order craft` inside a contract channel.
   * Inspect the rendered image preview on the contract's real roster.
   * For newly created orders, **SELECT** defaults to unselectable until you press **SAVE** and name the preset.
   * If an existing preset was loaded to view/edit, **SELECT** is immediately active.
   * Pressing **SELECT** changes the contract boost order to `ContractOrderCustom`, stores the custom order rules with the contract, and recalculates the boost order for when the contract starts.

---

## 12. Existing BoostBot Boost Orders in Custom Order Syntax

Every built-in boost order in BoostBot can be reproduced using the Custom Boost Order syntax. The table below maps each built-in order to its 4-line configuration:

| Built-in Order | Level 1 (Primary) | Level 2 (Tiebreaker 1) | Level 3 (Tiebreaker 2) | Level 4 (Tiebreaker 3) | Description |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **Sign-up** (`Signup`) | `SIGNUP` | — | — | — | Players boost in the order they joined the contract thread. |
| **Reverse** (`Reverse`) | `REVERSE` | — | — | — | Boosts in reverse order of joining (last to join boosts first). |
| **Random** (`Random`) | `RANDOM` | `SIGNUP` | — | — | Deterministic randomized order with sign-up order fallback. |
| **Boosting IHR** | `<IHR` | `<DEFL` | `<DELIV` | `<TE` | Highest IHR first, resolved by deflector rarity, delivery capacity, and TE. |
| **Fuzzy IHR** | `<IHR[6%]` | `<DEFL` | `<DELIV` | `<TE` | Highest IHR with a 6% tolerance band, allowing deflector quality to break close ties. |
| **Egg Laying Rate** (`ELR`) | `<ELR` | `SIGNUP` | — | — | Highest egg laying rate from equipped artifacts boosts first. |
| **Token-Ask** (`Token-Ask`) | `>TOKENS` | `SIGNUP` | — | — | Players requesting the fewest tokens boost earliest. |
| **Truth Eggs** (`TE`) | `<TE` | `SIGNUP` | — | — | Highest Prophecy / Truth Egg count boosts first. |
| **Fuzzy TE** | `<TE[sqrt]` | `SIGNUP` | — | — | TE count with square-root fuzzy banding to compress large PE gaps. |
| **Token Value** (`TVal`) | `<TVAL` | `>TOKENS` | `SIGNUP` | — | Highest historical Token Value boosts first, broken by fewest tokens wanted. |

### Recipes & Adaptations

#### 1. Role Tiered Order (Standard)
* **Custom Configuration**: Tiers roles (`SIAB > Gusset > Quant > Main > Helper`), prioritizes 2-slot deflectors (`T4L`, `T4E`) over 1-slot deflectors (`T4R`), applies 6% fuzzy banding to IHR, and uses tokens requested as the final tiebreaker.
* **Custom Syntax**:
  ```text
  Level 1: <ROLE
  Level 2: <DEFL_SLOT
  Level 3: <IHR[6%]
  Level 4: >TOKENS
  ```
* **Effort-Balanced Variant**:
  Replace Level 2 with balanced deflector crafting effort:
  ```text
  Level 1: <ROLE
  Level 2: <DEFL_EFFORT[50]
  Level 3: <IHR[6%]
  Level 4: >TOKENS
  ```

#### 2. Actuator-Focused Boost Order (Primary T4L, Tiebreaker Crafts)
* **Behavior**: Prioritizes players with a T4L Actuator. If players tie (both have or both lack a T4L Actuator), ties are broken by their total T4 Actuator craft attempts:
* **Custom Syntax**:
  ```text
  Level 1: COUNT(T4L_ACTUATOR)
  Level 2: CRAFT(T4_ACTUATOR)
  Level 3: 
  Level 4: 
  ```
