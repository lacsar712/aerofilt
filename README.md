# aerofilt

Wastewater aeration biofilter backwash coordination service written in Go.

## Overview

`aerofilt` coordinates multi-cell biofilter plants: head level monitoring, aeration zones, blower air scour, manifold valve actuation, and sequenced backwash phases. The service exposes a JSON HTTP API (no bundled frontend).

## Build and test

```bash
cd aerofilt
go build ./...
go test ./...
```

## Run

```bash
go run ./cmd/aerofilt
```

## Key flows

1. `app.RequestBackwash` acquires an interlock lease (`defer unlock`), then `backwash.Coordinator.Run` opens valves via `manifold.ValveBank.Open`.
2. `fsm.WashMachine.Transition` calls `backwash.Emitter.Emit`.
3. `config.WashCloseWindow` is enforced via `clock.ProcessClock` in `backwash.Window`.
4. `store.FilterStore.Snapshot` returns deep-copied slices.
