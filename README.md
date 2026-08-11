# Go event-sourced bank account

A Go port of [akhansari/EsBankAccount][upstream], the F# bank-account kata and
functional event-sourcing proof of concept.

The port keeps the original decider rules and user-visible workflow while
replacing Blazor, Elmish, Equinox, and the fake database with a small Go server
and a dependency-free browser UI.

## Run

The project uses Go 1.27 generic methods through [`go-functionalish`][library].
Until Go 1.27 is released, select Go 1.27 RC2 explicitly:

```sh
GOTOOLCHAIN=go1.27rc2 go run ./cmd/esbank
```

Open <http://localhost:8080>. Use `-addr` to choose another listen address.

All event streams and projections are volatile and reset when the process
stops. The default address is loopback-only. Binding another interface shares
one unauthenticated ledger with every client that can reach the server, so do
not expose this kata to an untrusted network.

## Domain rules

- Deposits and withdrawals emit one event.
- Withdrawals may overdraw unless a threshold is supplied to the domain decider.
- A threshold is inclusive: reaching it is allowed, crossing it is rejected.
- Closing a positive account emits a full-balance withdrawal followed by `Closed`.
- A negative account cannot close.
- Every command against a closed account is rejected.
- State is always reconstructed by folding the event history.

The HTTP boundary accepts only positive amounts with at most two decimal
places. The pure domain deliberately preserves the original kata behavior for
zero and negative amounts.

## Architecture

```text
bank/          pure Initial, Evolve, Fold, and Decide functions
app/           in-memory event streams and atomically updated read model
web/           standard-library JSON API and embedded browser application
cmd/esbank/    executable composition root
```

The application uses `go-functionalish` where the functional model benefits
from it:

- `result.Result` for accepted or rejected decisions
- `option.Option` for optional withdrawal thresholds
- `seq.Seq` to fold event history into aggregate state

No framework, reflection, database, frontend toolchain, or runtime CDN is required.

## Test

```sh
GOTOOLCHAIN=go1.27rc2 go test -race ./...
GOTOOLCHAIN=go1.27rc2 go vet ./...
```

The tests port the original event and state scenarios, then add projection,
command API, input-boundary, and error-path coverage.

## API

- `GET /api/accounts`
- `GET /api/account?id=<account>`
- `GET /api/commits`
- `POST /api/commands`

Command body:

```json
{
  "accountId": "checking",
  "type": "deposit",
  "amount": "25.00"
}
```

`type` is `deposit`, `withdraw`, or `close`. Close does not require an amount.

## Attribution

The original project and kata were created by Amin Khansari and released under
the MIT License. This port retains that copyright notice.

[library]: https://github.com/natalie-o-perret/go-functionalish
[upstream]: https://github.com/akhansari/EsBankAccount
