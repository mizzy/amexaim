# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

`amexaim` is a single-binary Go CLI that reads an American Express transaction CSV (Shift_JIS encoded, downloaded from the AMEX site) and posts each row as a payment to [Zaim](https://zaim.net/) via its API.

## Commands

```sh
go build ./...          # build the binary
go run . path/to.csv    # run against a file
cat path/to.csv | go run .   # run reading CSV from STDIN
go vet ./...
go fmt ./...
```

There is no test suite.

Runtime requires these env vars: `ZAIM_CONSUMER_ID`, `ZAIM_CONSUMER_SECRET`, `ZAIM_ACCESS_TOKEN`, `ZAIM_ACCESS_TOKEN_SECRET`, `ZAIM_ACCOUNT_NAME`, `ZAIM_CATEGORY`, `ZAIM_GENRE`. The names are resolved to numeric IDs against the Zaim account/category/genre lists at startup.

## Architecture

Everything lives in `main.go` (~220 lines). The flow is:

1. **Bootstrap** — construct a `gozaim.Client` from env vars, then call `FetchAccounts` / `FetchCategories` / `FetchGenres` to translate the configured names into IDs. Missing matches are fatal.
2. **Input** — open `os.Args[1]` if given, otherwise read from `os.Stdin`. The reader is wrapped in `transform.NewReader(..., japanese.ShiftJIS.NewDecoder())` because the AMEX CSV is Shift_JIS. The header row is skipped with a bare `reader.Read()`.
3. **Per-row processing** — for each CSV row:
   - Skip rows where `row[2] == "前回分口座振替金額"` (previous-period bank transfer; would double-count against the source bank account).
   - Parse `row[0]` (date, `YYYY/MM/DD` → `YYYY-MM-DD`), `row[2]` (place), `row[5]` (amount, strip commas).
   - `convertPlace` normalizes the place string: `?` → `ー` (the CSV mangles the chōonpu), full-width → half-width via `width.Fold`, trim spaces.
   - `Payment.Duplicated()` queries Zaim for same-day payments from the same account and treats it as a duplicate when amount matches AND the existing record's `Comment` or `Place` contains the new `Place` (substring, not equality).
   - `Payment.SetCategoryAndGenre()` looks back 3 months in Zaim history for a prior payment whose `Comment`/`Place` contains the current `Place`; if found, it overrides `CategoryID`/`GenreID` with the historical values so recurring merchants keep their previous categorization. Otherwise the defaults from `ZAIM_CATEGORY`/`ZAIM_GENRE` stand.
   - Post via `zaim.CreatePayment` with params built by `go-querystring`.

The `Money` struct mirrors Zaim's `FetchMoney` query params; the `url` struct tags are what `query.Values` reads.

## Conventions

- CSV column indices (`row[0]`, `row[2]`, `row[5]`) are hardcoded to the AMEX export format — changes there will silently misparse.
- Place-matching for both duplicate-detection and category inheritance is **substring containment in either direction** (new place in old comment/place, or old place in new). Keep both directions when editing this logic.
- Comments and a few log messages are in Japanese; preserve that style when editing nearby code.
