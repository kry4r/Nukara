# Mobile Persona & Proactive Refactor Design

**Date:** 2026-03-07
**Status:** Approved

## Summary

This refactor replaces the current mixed bot-persona model and responsive web shell with a phone-first product model that is consistent across platforms. The app will always render inside a centered 9:16 mobile viewport, bot definitions will be promoted to a new five-part schema, proactive messaging frequency will become explicit time-based configuration, and runtime prompts will gain locale awareness derived from the bot's life context.

## Goals

- Render the entire web app as a fixed 9:16 phone screen on every platform without stretch scaling.
- Replace the legacy bot definition presentation with five first-class fields:
  - identity
  - personality
  - expression_style
  - life_context
  - taboos_and_preferences
- Stop fragmenting bot replies with fixed rune windows; instead combine prompt guidance plus semantic message splitting that feels like natural WeChat chat.
- Fix the "bot not found" self-iteration flow from the contacts/bot detail path.
- Replace `high/normal/low` proactive frequency with explicit user-selectable intervals from 10 minutes to 5 hours.
- Make DND independently effective and easy to verify.
- Infer local time and regional context from `life_context`; default to China when no location is present.
- Add regression coverage, including Playwright MCP flows.

## Non-Goals

- Removing all legacy persona fields in this iteration.
- Introducing free-form custom proactive intervals.
- Building a full geolocation/NLP service; location inference remains lightweight and deterministic.

## Product Decisions

### 1. Fixed 9:16 app shell

- Every platform renders a centered mobile canvas with a fixed 9:16 aspect ratio.
- The browser page acts as a background stage only.
- Internal page regions scroll; the outer browser page should not become the main scroll surface.
- Safe-area insets remain supported inside the phone shell.

### 2. Bot definition model

The app standardizes on five user-facing fields:

1. `identity` — who the bot is and how it relates to the user
2. `personality` — stable personality traits
3. `expression_style` — speaking rhythm, tone, wording habits, conversational style
4. `life_context` — living environment, country/region, routine, cultural context
5. `taboos_and_preferences` — boundaries, dislikes, preferred topics/forms of address

These become the primary fields across create/edit/detail/iteration APIs and UI.

### 3. Reply generation and splitting

Naturalness is controlled in two layers:

- **Prompt layer**: the system prompt explicitly tells the model to decide whether a response should be one message or several short messages, like a real WeChat user.
- **Delivery layer**: the backend stops fixed-size rune slicing and instead:
  - respects explicit multi-message protocol output when present
  - otherwise splits by semantic boundaries first (line breaks, punctuation, pause markers)
  - only falls back to length windows when no better split exists

The model should avoid meaningless micro-fragments while still sounding human.

### 4. Proactive frequency and DND

- Replace `high/normal/low` with explicit intervals:
  - 10m, 20m, 30m, 45m, 1h, 2h, 3h, 4h, 5h
- Frequency and DND are independent:
  - frequency = minimum gap between proactive sends
  - DND = absolute block window for automated proactive sends
- DND must not block normal replies to user-initiated messages.
- Manual debugging/test APIs should return clear reasons when proactive send is blocked.

### 5. Local time and regional behavior

- Parse `life_context` for country/city hints.
- Map those hints to a timezone and region prompt helper.
- Default to China (`Asia/Shanghai`) if no location is found.
- Inject runtime context such as local clock, day phase, and region note so the bot responds according to where it supposedly lives.

## Data Model and Migration

## New bot fields

Extend the backend `Bot` model and `bots` table with:

- `identity_text` TEXT NOT NULL DEFAULT ''
- `personality_json` JSONB NOT NULL DEFAULT '[]'
- `expression_style_text` TEXT NOT NULL DEFAULT ''
- `life_context_text` TEXT NOT NULL DEFAULT ''
- `taboos_preferences_text` TEXT NOT NULL DEFAULT ''

## Legacy compatibility

Legacy fields remain temporarily for compatibility during the refactor:

- `summary`
- `relationship`
- `role`
- `self_cognition`
- `speaking_style`
- `background`
- `traits`

The new fields become the system of record for all new UI and API flows. Compatibility helpers backfill legacy fields where old prompt or API paths still depend on them.

## Backfill strategy

A migration/backfill step should populate new fields from existing data:

- `identity_text <- relationship + role + summary`
- `personality_json <- traits`
- `expression_style_text <- speaking_style`
- `life_context_text <- background`
- `taboos_preferences_text <- self_cognition`

Backfill should be idempotent and skip overwriting non-empty new fields.

## API Design

## Bot CRUD

Update bot create/read/update payloads to center on the new schema.

### Create/update request shape

```json
{
  "name": "...",
  "identity": "...",
  "personality": ["..."],
  "expression_style": "...",
  "life_context": "...",
  "taboos_and_preferences": "...",
  "gender": "unknown",
  "avatar_base64": "..."
}
```

### Read payload shape

All bot reads should return the five new persona fields. Legacy keys may remain during the transition but should no longer be the primary frontend contract.

## Profile and iteration endpoints

- `/api/v1/bots/:id/profile` returns the new persona fields plus state, directives, and conversation information.
- `/api/v1/bots/:id/iterate` returns patch data against the new model, for example:

```json
{
  "identity_adds": ["..."],
  "personality_adds": ["..."],
  "expression_style_adds": ["..."],
  "life_context_adds": ["..."],
  "taboos_and_preferences_adds": ["..."],
  "bot": {}
}
```

## Contacts/detail iteration bug fix

The self-iteration action must use the canonical loaded bot id and must not fail simply because the bot lacks an existing conversation. Backend logic should distinguish:

- missing bot
- missing conversation
- empty message history

and degrade gracefully for the latter two.

## Frontend Design

## App shell

- Replace the current max-width responsive root with a fixed 9:16 centered shell in `App.vue` and global CSS.
- The phone shell becomes the consistent layout boundary for all pages.

## Bot form/detail/list

- `BotFormView.vue` switches fully to the five new fields.
- `BotDetailView.vue` displays the five fields instead of `简介 / 说话风格 / 背景 / 特质`.
- `BotsView.vue` uses a compact identity summary derived from the new model.
- Iteration result display also uses the new field names.

## Settings

- Replace the current three-tier frequency select with explicit interval choices from 10 minutes to 5 hours.
- Keep DND controls separate and visually clear.
- Show save state/feedback for both frequency and DND.

## Runtime Prompting and Delivery

## Persona prompt compiler

Introduce a new persona compiler layout that uses the five new fields directly. Legacy prompt helpers should read from compatibility conversions until fully migrated.

## Chat style prompt rules

Extend runtime prompt rules to include:

- decide whether to send one message or multiple short messages
- split only when that improves realism
- avoid mechanical micro-splitting
- keep total reply length proportional to the user's input

## Multi-message delivery protocol

Prefer a deterministic internal protocol such as:

- JSON array of message parts, or
- a reserved separator token not shown to the user

Fallback semantic splitting should process punctuation, line breaks, and natural pause words before any hard length slicing.

## Locale inference

Add a lightweight location inference helper that:

- scans `life_context`
- matches a supported country/city dictionary
- returns timezone, locale label, and day-phase context
- defaults to China if nothing is matched

Inject the inferred values into runtime system context for both normal and proactive replies.

## Proactive Scheduler Changes

- Replace `frequencyCooldown` enum mapping with explicit minutes-based intervals from notification settings.
- Update storage and API contract accordingly.
- Ensure DND checking happens before proactive delivery.
- Include blocked reasons in manual test endpoints (`proactive_disabled`, `dnd_active`, `cooldown_active`, etc.).

## Testing Strategy

## Backend tests

Add or update tests for:

- migration/backfill of new persona fields
- bot CRUD using the new payload contract
- persona compiler output using the new fields
- iteration patch parsing and application
- semantic reply splitting behavior
- locale inference from `life_context`
- proactive cooldown using explicit minute values
- DND enforcement in scheduler/manual proactive path
- graceful handling when bot exists but conversation/history is missing

## Frontend tests

At minimum cover:

- fixed 9:16 app shell rendering assumptions
- new bot form fields
- bot detail rendering using new persona fields
- settings interval options from 10 minutes to 5 hours

## Playwright MCP verification

Run end-to-end verification after implementation for:

- desktop viewport still renders a fixed 9:16 phone shell
- mobile viewport still renders the same fixed-ratio shell
- bot create/edit/detail flow with the five new persona fields
- self-iteration action no longer surfaces `bot not found`
- settings page supports the new interval options and DND persistence
- chat replies stream in natural message groups rather than arbitrary tiny fragments

## Risks and Mitigations

- **Schema drift risk**: use additive migration plus compatibility helpers.
- **Prompt regression risk**: protect with persona compiler and chat-splitting unit tests.
- **Frontend contract mismatch**: migrate API and UI together, then verify with Playwright MCP.
- **Location inference brittleness**: keep inference deterministic, small-scope, and default to China.

## Rollout Notes

- This is a full domain refactor but should still be delivered incrementally behind compatible helpers.
- Legacy field deletion should happen only after the new model is stable in production and all callers have migrated.
