# Expedition Carrier Commands Design

## Overview

Fleet carrier tracking system for Elite Dangerous expeditions. Tracks carrier locations, jump schedules, destinations, and status messages.

## Configuration

### Config Structure

```json
{
  "carrierOwnerIds": [
    "discord-user-id-1",
    "discord-user-id-2"
  ],
  "carriers": [
    {
      "stationId": "W7H-6DZ",
      "inaraId": 184006,
      "name": "DSEV Odysseus"
    }
  ],
  "carrierLocationCacheMinutes": 20,
  "botChannels": ["channel-id-1"]
}
```

### Permission Model

- `ownerIds` (existing) + secure channel: full access to all bot commands
- `carrierOwnerIds`: can use carrier management commands from any channel
- Everyone else: can use `!carriers` / `/carriers` to view info

### Config Rename

Rename `cooldownWhitelistChannels` to `botChannels`. Used for:
- Cooldown bypass (existing behavior)
- `!carriers` replies in channel instead of DM

## Database Schema

### Table: `carriers`

| Column | Type | Description |
|--------|------|-------------|
| station_id | TEXT PRIMARY KEY | Carrier callsign (e.g., "W7H-6DZ") |
| current_system | TEXT | Current location from Inara |
| location_updated | INTEGER | Unix timestamp of last Inara fetch |
| jump_time | INTEGER | Unix timestamp of next/last jump (NULL if none) |
| destination | TEXT | Target system name (NULL if none) |
| status | TEXT | Free-form status text (NULL if none) |

Notes:
- Carrier definitions (name, Inara ID) live in config
- Database stores runtime state only
- Rows created on first interaction (lazy initialization)

## Commands

### Message Commands

| Command | Syntax | Permission | Description |
|---------|--------|------------|-------------|
| `!carrierjump` | `!carrierjump <station-id> <unix-timestamp>` | Commanders | Set next jump time |
| `!carrierdest` | `!carrierdest <station-id> <system>` | Commanders | Set destination |
| `!carrierstatus` | `!carrierstatus <station-id> <free text>` | Commanders | Set status message |
| `!carrierclear` | `!carrierclear <station-id> jump\|dest\|status\|all` | Commanders | Clear field(s) |
| `!carriers` | `!carriers` | Everyone | Show all carriers |

### Slash Commands

| Command | Options | Description |
|---------|---------|-------------|
| `/carrierjump` | `carrier` (autocomplete), `timestamp` (integer) | Set next jump time |
| `/carrierdest` | `carrier` (autocomplete), `system` (string) | Set destination |
| `/carrierstatus` | `carrier` (autocomplete), `status` (string) | Set status message |
| `/carrierclear` | `carrier` (autocomplete), `field` (choice) | Clear field(s) |
| `/carriers` | (none) | Show all carriers (ephemeral) |

Autocomplete provides carrier names + station IDs from config.

## Architecture

```
┌─────────────────┐     ┌─────────────────┐
│ Message Handler │     │ Slash Handler   │
└────────┬────────┘     └────────┬────────┘
         │                       │
         └───────────┬───────────┘
                     ▼
         ┌───────────────────────┐
         │   Carrier Service     │
         │ (shared core logic)   │
         └───────────────────────┘
```

Both handlers call shared service functions.

## Inara Location Fetching

### Strategy

- Scrape `https://inara.cz/elite/station/<inaraId>/` for current system
- Cache result in database with timestamp
- Re-fetch if cache older than `carrierLocationCacheMinutes`

### Cache Flow

```
For each carrier:
  1. Check database for cached location
  2. If no cache OR cache expired:
     - Fetch from Inara (HTTP GET + parse HTML)
     - Update database
  3. Return location
```

### Error Handling

- Inara unreachable: show cached location with "(cached)" or "Location unknown"
- Parsing fails: log error, return "Location unavailable"
- Consider global fetch rate limit (max 1 req/sec)

## Output Format

### Carrier List Response

```
OFFICIAL FLEET CARRIERS
Times shown in your local time
Linked to Inara for up-to-date information

DSEV ODYSSEUS - W7H-6DZ
📍 Kyloall CL-Y g3 (last updated <t:1737402600:R>)
🚀 Departed <t:1737399000:F> (<t:1737399000:R>)
📌 Destination: Blae Drye XO-Z d13-0
Please remain seated while the carrier is in transit

DSEV DISTANT SUNS - V2W-85Z
📍 Kyloall CL-Y g3
⏱️ Departing <t:1737410000:F> (<t:1737410000:R>)
📌 Destination: Blae Drye XO-Z d13-0
ℹ️ Boarding now - departure imminent

DSEC FIMBULTHUL - V4V-2XZ
📍 Kyloall CL-Y g3
No scheduled jump
```

### Discord Timestamp Formats

- `<t:timestamp:F>` - Full date/time
- `<t:timestamp:R>` - Relative time

### Display Logic

- Jump time in past → "Departed" + "Please remain seated..."
- Jump time in future → "Departing"
- No jump time → "No scheduled jump"
- Destination shown only if set
- Status shown only if set

### Response Location

- Message `!carriers` in `botChannels` → reply in channel
- Message `!carriers` elsewhere → DM
- Slash `/carriers` → ephemeral (always)

## Transit State

Jump time in the past = "in transit" state.

Clears when: a new future jump time is set.

Does NOT auto-clear based on time or location change.

## File Structure

```
core/
├── settings.go              # Add carrier config fields + getters
├── database/
│   └── carriers.go          # Carrier table schema + CRUD
└── dispatch/
    └── handlers/
        └── carriers.go      # Message command handlers + service
        └── carriers_slash.go # Slash command handlers + registration

main.go                      # Register slash commands on startup
config.json.example          # Add carrier config examples
config-dev.json              # Add expedition carriers
```

### Service Functions

- `GetCarrierInfo(stationId)` - fetch/cache location, return full state
- `SetJumpTime(stationId, timestamp)`
- `SetDestination(stationId, system)`
- `SetStatus(stationId, text)`
- `ClearField(stationId, field)`
- `GetAllCarriers()` - returns formatted carrier list
- `FetchInaraLocation(inaraId)` - HTTP fetch + parse

## Future: EDDN Integration

Database schema supports EDDN updates. Implementation deferred.

When added:
- Background goroutine connects to EDDN on startup
- Listens for carrier location updates
- Updates `current_system` and `location_updated` in real-time

## Carriers

Initial carriers to add:

| Name | Station ID | Inara ID |
|------|------------|----------|
| DSEV Odysseus | W7H-6DZ | TBD |
| DSEV Distant Suns | V2W-85Z | TBD |
| DSEC Fimbulthul | V4V-2XZ | TBD |
| Pillar of Chista | TBQ-6VX | TBD |