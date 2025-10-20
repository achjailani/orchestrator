# A Tiny Orchestrator for Go
A tiny and type-safe Saga Pattern orchestrator for managing distributed transactions in Go. Built with generics, resumable execution, and optional persistence—perfect for microservices, event-driven workflows, and reliable business processes.
> “Failures are inevitable. Data inconsistency shouldn’t be.”

### Features
* Resumable sagas: Automatically resume from the last failed step after crashes or restarts.
* Idempotent by design: Safe to retry with the same idempotency key.
* Optional persistence: Plug in any storage backend (or run entirely in-memory).
* Type-safe workflows: Full generics support—your business data stays strongly typed.
* Compensation support: Define undo logic for each step to maintain consistency.
* Skip & continue-on-error: Flexible control over step execution flow.
* Zero external dependencies: Pure Go standard library.

## Quick Start

## License
Licensed under [MIT License](./LICENSE)