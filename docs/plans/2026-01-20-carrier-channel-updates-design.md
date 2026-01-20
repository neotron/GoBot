# Carrier Channel Updates Design

## Overview

Watch a dedicated Discord channel for carrier status messages and automatically update carrier data (departure time, destination) when messages are edited.

## Configuration

New config option:
```json
{
  "carrierUpdateChannelId": "123456789"
}
```

## Message Format

```
Carrier: <name> <STATION-ID>
Current System: <system or "In Transit">
Destination System: <destination>
Departure: <time in UTC format>
Tritium Buy Orders: <info>
```

Multiple carrier blocks may appear in a single message, separated by blank lines.

## Implementation

### File Structure

- `core/settings.go` - Add `carrierUpdateChannelId` config
- `core/services/carrier_updates.go` - Parser and update logic
- `main.go` - Hook into `messageUpdate` handler

### Data Structures

```go
type CarrierUpdate struct {
    StationId   string
    Departure   *int64  // parsed unix timestamp
    Destination *string // raw parsed value
}
```

### Parsing Logic

1. Split message content by "Carrier:" to find carrier blocks
2. For each block:
   - Extract station ID using XXX-XXX pattern regex
   - Extract Departure line, parse with `ParseJumpTime()`
   - Extract Destination System line

### Update Logic

1. Check channel matches `carrierUpdateChannelId`
2. Check author is in `carrierOwnerIds`
3. Parse message content for carrier blocks
4. For each carrier:
   - Skip if station ID not in config
   - Fetch current database state
   - Update Departure if changed
   - Update Destination if changed AND valid system (verified via EDSM)
5. Log changes at INFO level

### Validation

- **Station ID:** Must exist in configured carriers
- **Departure:** Must be parseable time format
- **Destination:** Must be valid system (EDSM returns coordinates)

### Edge Cases

- Empty/missing field → skip that field (don't clear existing)
- Unparseable time → log warning, skip field
- "[processing]" or "In Transit" → skip field
- Unknown station ID → log debug, skip carrier

## Not In Scope (Future)

- Current System parsing
- Tritium Buy Orders
- Status field extraction
