# Custom Boost Orders

Boost Bot supports **Custom Boost Orders**, allowing contract coordinators to create multi-tier sorting rules using a concise comparison language. Custom orders can be tested, previewed via a rendered image table, saved as personal presets, published globally across servers, or applied directly to a live contract.

---

## 1. Defining a Custom Boost Order (`/define-custom-order`)

Run the `/define-custom-order` command:

```text
/define-custom-order [order]
```

### Autocomplete Options
The `order` parameter autocomplete provides:
* `<NEW>`: Create a new custom boost order from scratch.
* `[User] <name>`: Load a personal preset previously saved to your profile.
* `[Global] <name>`: Load a globally published preset accessible across all servers.

---

## 2. The 4-Line Criteria Modal

Selecting `<NEW>` or clicking **MODIFY** opens the **Custom Boost Order Criteria** modal with 4 rows:

| Row | Role | Required | Example |
| :--- | :--- | :---: | :--- |
| **Level 1** | Primary sorting rule | Yes | `<DEFL_EFFORT[50]` |
| **Level 2** | Secondary tiebreaker | No | `<IHR[6%]` |
| **Level 3** | Tertiary tiebreaker | No | `>TOKENS` |
| **Level 4** | Quaternary tiebreaker | No | `<TE` |

> [!NOTE]
> Higher levels take absolute precedence. Lower levels are only evaluated when players are **tied** on all preceding levels. If all 4 levels result in a tie, sign-up order is used as the fallback.

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

## 6. Complete Criteria Reference

| Symbol | Meaning | Default Direction | Description & Usage |
| :--- | :--- | :---: | :--- |
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
| `DELIV` | Delivery Capacity | `<` (Highest) | Estimated maximum shipping delivery rate. |
| `SIGNUP` | Sign-up Order | `<` (First-in) | Order in which players signed up in the contract thread. |
| `RANDOM` | Deterministic Random | — | Stable pseudo-random tiebreaker. |

---

## 7. ⚙️ Preview & Evaluation Report

Submitting the criteria modal or loading a preset generates an interactive preview report containing:
1. **Tiebreaker Hierarchy**: Shows the exact condition configured for Levels 1–4.
2. **Table Image Preview**: Renders a PNG table displaying each booster's data (`#`, `Player`, `Deflector`, `T4 Crafts`, `Effort [N]`, `ELR`, `IHR`, `Tokens`, `TE`).
   * Outside a contract channel: Evaluates a 6-player benchmark cohort.
   * Inside a contract channel: Evaluates the active contract's actual players.

### Action Buttons:
* **MODIFY**: Re-opens the 4-row criteria modal to tweak rules.
* **SAVE**: Opens a modal to enter a Name and saves the preset to your personal profile.
* **PUBLISH**: Opens a modal to enter a Name and publishes the preset globally across all servers.
* **APPLY**: *(Only in active contracts)* Immediately applies the custom order to the contract boost list.
* **DISMISS**: Closes the preview report.

---

## 8. Applying Custom Orders in Contracts

1. **Via Dropdown (`cs_#order`)**:
   * In contract signup or `/contract-settings`, open the **Boosting Order** select menu.
   * All published global orders (`⚙️ [Global] <name>`) and personal presets (`👤 [User] <name>`) appear in the list.
2. **Via `/define-custom-order`**:
   * Run `/define-custom-order` inside a contract channel.
   * Inspect the rendered image preview on the contract's real roster.
   * Click **APPLY** to apply the order and update the boost list.
