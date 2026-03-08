# Issue #15 Memory, Persona, and Living-State Design

**Date:** 2026-03-08
**Status:** Approved with revisions
**Issue:** `#15 [feat] 记忆与机器人人设功能增强`
**Worktree:** `/Users/nidhogg/code/Nukara/.worktrees/issue-15-memory-persona`
**Main Baseline Reviewed:** `bb0b6bc` (`feat: add admin memory graph and email auth flow`, 2026-03-08)

## Summary

This design upgrades the current bot runtime from a mostly static persona responder into a lightweight continuous character system. The bot should preserve key memories, honor chat-local promises, maintain a believable current-life state, and expose recent changes clearly in both the user bot-detail page and the existing admin memory tooling.

The design intentionally avoids a heavy simulation engine. Instead, it separates stable persona from long-term memories, recent events/promises, and current living state. Runtime prompting injects only a compact state summary plus a small set of relevant memories. Post-turn processing runs asynchronously through a lightweight pipeline that can use a cheaper dedicated model, configured separately from the main chat model and managed from Admin.

## Goals

- Support four first-class memory categories:
  - `event`
  - `promise`
  - `self_fact`
  - `user_fact`
- Let the bot answer questions like “你在干什么” with believable current-life behavior rather than mechanically repeating job/identity text.
- Preserve continuity across turns and sessions without making prompt context too heavy.
- Implement chat-only promise fulfillment for the first iteration.
- Auto-write low-risk changes and facts, while routing core persona changes to confirmation.
- Move `impression` and `iterate` from page-triggered generation to chat-driven asynchronous updates.
- Rework the user detail page into a runtime portrait view.
- Extend the new Admin memory UI and handlers from `main` instead of creating parallel tooling.
- Allow the lightweight post-turn pipeline to use a separately configured lower-cost model, with Admin-side configuration support.

## Non-Goals

- Building a full daily schedule simulator or world simulation engine.
- Implementing external reminders, push notifications, or non-chat execution of promises.
- Supporting multi-user shared bot state. The working assumption is one user owns and chats with their own bots.
- Replacing the entire existing memory stack or graph architecture.
- Requiring every chat turn to run expensive summarization or large-memory recall.

## Confirmed Product Decisions

### 1. Scope is end-to-end

The first release covers:

- memory extraction and persistence
- runtime reply shaping
- current-life state continuity
- chat-only promise follow-through
- detail-page presentation
- admin inspection/configuration alignment

### 2. Living-state strength

The bot should have a **medium-strong living state**, but not a full simulation. It should feel like a person with ongoing activity, routine, and continuity, while staying cheap enough to operate and simple enough to reason about.

### 3. Promise execution boundary

Promises are only fulfilled **inside chat** in the first release. The system should remember and naturally bring them back up when relevant, but should not trigger out-of-band reminders yet.

### 4. Risk boundary

Only **core persona changes** are high-risk. These require explicit confirmation before merging into stable persona fields. Low-risk lifestyle facts, recent impressions, temporary states, promises, and important events can be auto-written.

### 5. User-mentioned memory boundary

`user_fact` memories are stored only when the user **explicitly states something that will affect future interaction**. Casual chatter, weak implications, and vague one-off remarks are ignored.

### 6. Data ownership

This design targets **user-owned bots**. Runtime state, memories, and self-evolution are tracked per `user_id + bot_id`, not as globally shared bot state across users.

## Core Design Principle

The current issue exists because the system often treats “who the bot is” as identical to “what the bot is doing right now.” That is incorrect. A believable character system needs separate layers for:

- stable persona
- long-term facts/habits
- recent events and promises
- current living state

The bot’s job, role, or identity may constrain the answer, but should not directly become the answer.

## Data Model

### Layer 1: Stable persona

Represents who the character is.

Fields continue to map to the existing persona v2 shape:

- `identity`
- `personality`
- `expression_style`
- `life_context`
- `taboos_and_preferences`

Characteristics:

- stable
- low-frequency change
- high-risk to mutate
- system of record for character identity

### Layer 2: Long-term memory

Stores durable facts that shape future behavior but are not the core persona definition.

Examples:

- stable habits
- long-term preferences
- durable relationship facts
- recurring self-described lifestyle patterns
- verified user facts that matter for future interaction

Characteristics:

- medium-frequency updates
- mostly low- or medium-risk
- retrieved selectively by relevance

### Layer 3: Events and promises

Stores important recent or medium-term developments.

Examples:

- important personal event
- something the user told the bot that matters soon
- bot-made promise
- short-term plan
- follow-up obligation

Characteristics:

- time-sensitive
- needs status transitions
- can expire or be fulfilled

### Layer 4: Current living state

Stores the bot’s current or near-current ongoing situation.

Examples:

- “刚下晚班，在回去路上”
- “今天休息，刚醒，脑子还没清醒”
- “在便利店值晚班，刚忙完一阵”

Characteristics:

- short-lived
- refreshed often
- derived from time-of-day + recent events + habits + recent chat
- must not be confused with stable persona

## Storage Strategy

### Reuse `memory_items` as the main fact/event store

The existing `store.MemoryItem` and `memory_items` table already provide a useful base:

- `kind`
- `owner`
- `content`
- `importance`
- `occurred_at`
- `status`
- `topics`

The first version should continue using `memory_items` as the main record for extracted facts, habits, events, and promises rather than splitting everything into many new tables.

### Extend `kind`

Recommended normalized values:

- `event`
- `promise`
- `self_fact`
- `user_fact`
- `habit`
- `state_basis`

Notes:

- `self_fact` covers bot-originated lifestyle facts.
- `habit` is a more durable subset once repeated/verified.
- `state_basis` can be used for low-level current-state evidence when needed, without making current state itself a plain memory line.

### Extend `status`

Recommended values:

- `active`
- `pending_confirm`
- `fulfilled`
- `expired`
- `rejected`
- `archived`

Notes:

- `promise` records will move through status transitions.
- high-risk candidate persona changes should not be mixed into active stable persona data.

### Add a dedicated runtime-state record

Current living state should not be represented only as plain memory text. It needs a dedicated result object for:

- current activity text
- state source labels
- confidence/strength if desired
- last updated timestamp
- optional related memory IDs

This can be implemented by either:

- extending the current bot-state concept carefully, or
- adding a dedicated `bot_runtime_states` store/table

Recommendation: **use a dedicated runtime-state structure** so the existing emoji/text status system is not overloaded with a new meaning.

### Add a persona-change event log

A separate change-log structure should store:

- change type
- target persona field
- proposed additions/modifications
- source turn references
- risk level
- state: accepted / pending / rejected / revoked
- created/updated timestamps

This log powers both:

- user-facing “自我迭代 / 最近变化” cards
- confirmation flows for high-risk stable persona changes

## Memory Classification and Write Rules

## First-class memory categories

### 1. `event`

Store important concrete events that affect later conversation.

Examples:

- the bot says it argued with a coworker
- the user says they have an exam next week
- the bot says it is moving soon

### 2. `promise`

Store explicit commitments made during chat.

Examples:

- “我明晚告诉你结果”
- “下次我接着讲这件事”

### 3. `self_fact`

Store bot-originated statements that form continuity.

Examples:

- sleep habit
- food routine
- work rhythm
- a recent self-described lifestyle constraint

### 4. `user_fact`

Store only explicit user-stated facts that matter for future interaction.

Examples:

- exam schedule
- dislike of late-night messages
- family/pet facts
- explicit important plans

Rejected examples:

- casual greetings
- vague mood in one message
- weakly implied preferences
- low-value social filler

## Filtering rules

A candidate should be dropped if it is:

- vague and not reusable
- only polite small talk
- obvious joke or rhetorical fluff
- unrelated to future interaction
- unlikely to matter after the current moment

A candidate is worth storing only if it affects at least one of:

- future replies
- character continuity
- promise follow-through
- current living-state inference

## Risk routing

### Auto-write

These can be written automatically:

- recent impressions
- current living-state updates
- low-risk self facts
- important recent events
- chat promises
- explicit low-risk user facts

### Auto-write but visibly recent/revocable

These may auto-write but should remain visible in “recent changes”:

- emerging habits
- stronger expression tendencies
- medium-term lifestyle tendencies

### Require confirmation

Only changes that mutate **stable persona identity** are considered high-risk.

Examples:

- identity rewrite
- core personality rewrite
- deep background rewrite
- hard taboo/preference rewrite

These should go into the persona-change event log as `pending` and must not silently overwrite stable persona fields.

## Runtime Prompting and Continuity

## Runtime answer priority

When the user asks about the bot’s current situation, answer generation should follow this priority:

1. current living state
2. recent event / due promise
3. long-term habit or durable self fact
4. stable persona as the last fallback

This ensures “I work at a convenience store” no longer automatically becomes “I am currently working at the convenience store.”

## Minimal runtime injection

Each chat turn should inject only a compact runtime package:

- stable persona summary
- current living-state summary
- 2-4 relevant long-term memories
- 0-3 due/relevant events or promises

This replaces all-memory injection with targeted context.

## Trigger-aware retrieval

The recall layer should bias memory selection by question type.

Examples:

- asking “你在干什么” favors current state + recent events + habits
- asking “你还记得我说过什么吗” favors `user_fact`
- asking “你不是答应过我吗” favors `promise`
- asking emotionally relational questions can include recent impression snippets

## Consistency guardrail

Before final reply generation, runtime prompt assembly should provide a light consistency block:

- current local time and day phase
- recent living state
- avoid contradicting the last known state without cause
- if evidence is weak, answer vaguely but plausibly rather than confidently hallucinating

This avoids obvious errors like:

- Sunday midnight described as normal daytime office work
- saying the bot is asleep and then immediately describing lunch with coworkers
- repeating the profession label as if it were a present-tense action

## Current Living-State Engine

The current living state should be updated as a lightweight derived result, not a full planner.

### Inputs

- current local time / day phase
- stable persona and life context
- recent events
- due promises
- repeated habits
- latest chat signal

### Update policy

- keep the previous state when still plausible
- update when a new event, promise, or explicit statement makes a change more likely
- prefer continuity over randomness
- keep the stored state short, human-readable, and explainable

### Result style

Stored state text should be natural and concrete, e.g.:

- “刚收完晚班那阵忙，现在在店后门喘口气”
- “白天补觉刚醒，整个人还有点懵”
- “刚回到住处，正泡面准备洗漱”

## Promise Fulfillment Model

The first release supports **chat-only** promise fulfillment.

Each promise should track:

- content
- originating turn/message
- due hint or condition if present
- status: `active/pending`, `fulfilled`, `expired`
- last surfaced timestamp if needed

Rules:

- surface due promises when directly relevant
- surface promises when the user follows up on them
- do not force every due promise into every unrelated answer
- update promise status once fulfilled in-chat

## Post-Turn Processing Pipeline

A lightweight asynchronous pipeline should run after the main reply is generated.

### Stage A: Candidate extraction

Inputs:

- user prompt
- bot reply
- recent compact history
- current system/runtime context

Outputs:

- candidate events
- candidate promises
- candidate self facts
- candidate user facts
- candidate persona changes

### Stage B: Classification and risk routing

Each candidate is classified into one of:

- long-term memory
- event/promise
- current-state update basis
- persona-change pending item

Risk routing decides:

- auto-write
- auto-write + visible change log
- pending confirmation

### Stage C: Deduplication and merge

Responsibilities:

- merge near-duplicate memories
- extend existing promise state rather than duplicating it
- strengthen repeated habits instead of storing every variant separately

### Stage D: Runtime refresh

Responsibilities:

- update current living state
- update promise states
- refresh recent impression when cadence or significance requires it

### Stage E: Summary outputs

Write user/admin-facing artifacts:

- recent impression summary
- recent change log items
- pending persona confirmations

## Dedicated Model Configuration for Lightweight Post-Turn Work

The post-turn pipeline should support its **own model configuration**, separate from the main chat model.

### Why

Post-turn work is structurally different from reply generation:

- extraction
- classification
- summarization
- lightweight state updates

These are good candidates for a cheaper/smaller model than the main conversational model.

### Configuration requirements

- add a dedicated config group for post-turn processing (for example `memory_pipeline` or `post_turn_pipeline`)
- allow separate provider, model, and optional temperature/limits if supported by the existing config system
- preserve a safe fallback to the main model if the dedicated model is missing
- expose this config in Admin alongside existing model/provider configuration surfaces

### Admin requirements

Admin should be able to:

- view the active post-turn provider/model
- update it without code changes
- understand that it drives memory extraction / impression / iteration summarization, not the live chat reply itself

This should extend the current Admin configuration work already present on `main`, not create a separate disconnected path.

## User Detail Page Redesign

The current bot detail page should become a **runtime portrait page**.

### Primary cards

#### 1. Current living state

Show:

- current activity/state text
- updated timestamp
- basis tags such as `time`, `event`, `habit`, `promise`

#### 2. Stable persona archive

Keep the existing five-field persona presentation, but present it as stable identity rather than current activity.

#### 3. Key memories

Grouped sections:

- important events
- open/recent promises
- long-term habits / durable facts
- important user facts

#### 4. Recent changes / self-iteration

Each item should show:

- what changed
- change type
- source turn(s)
- risk level
- status: active / pending / rejected / revoked

#### 5. Recent impression

Show 1-3 short refreshed summaries, sourced from asynchronous chat-driven updates rather than a page-triggered generation button.

### High-risk confirmation card

A dedicated card should list pending core persona changes with:

- target field
- proposed change
- source evidence
- accept / reject actions

### Directive handling

`行为指令` should no longer be a central top-level concept on the detail page. If the underlying directive mechanism still exists, it can move into an advanced/system area.

## Admin Integration

The new `main` branch already contains Admin memory graph support:

- `Nukara_Admin_Web/src/components/MemoryGraphPanel.vue`
- `Nukara_Backend/internal/admin/memory_graph_handler.go`

This design should extend that work rather than creating a parallel admin page.

### Admin memory additions

Recommended additions include:

- filter by `kind`
- filter by `status`
- inspect current living-state record
- inspect recent impression summary
- inspect recent change events
- inspect pending persona confirmations

Admin is the inspection/debugging surface for the underlying memory/state system. The user-facing detail page is the product surface.

## API Design

## Extend `/api/v1/bots/:id/profile`

This should become the primary read endpoint for the bot runtime portrait page.

Recommended response sections:

- `bot`
- `bot_state` (existing lightweight status if still needed)
- `runtime_state`
- `recent_impressions`
- `key_memories`
- `recent_changes`
- `pending_persona_changes`
- `conversation_id`

This lets the detail page load from one main request.

## Reframe `/impression`

The current `/api/v1/bots/:id/impression` endpoint should stop being the primary source of freshly generated text. It should instead:

- return already-computed impression data, or
- become an admin/debug endpoint for forced regeneration only

## Reframe `/iterate`

The current `/api/v1/bots/:id/iterate` endpoint should stop being a user-facing button that triggers the main experience. It should instead:

- return stored recent changes, or
- exist only as a maintenance/debug path for recomputation

## Confirmation endpoints

Add explicit endpoints for accepting/rejecting pending core persona changes.

Examples:

- `POST /api/v1/bots/:id/persona-changes/:changeID/accept`
- `POST /api/v1/bots/:id/persona-changes/:changeID/reject`

## Performance and Cadence

To keep runtime cheap:

### Every turn, lightweight only

- candidate extraction
- promise status update
- small current-state refresh when needed

### Every 3-5 turns or on important triggers

- recent impression refresh
- self-iteration summary refresh
- habit consolidation

### Immediate on significant triggers

- explicit promise
- important event
- explicit self-fact with continuity impact
- candidate core persona mutation

## Testing Strategy

## Backend tests

Extend existing coverage around:

- `Nukara_Backend/internal/api/bot_profile_test.go`
- `Nukara_Backend/internal/api/ws_chat_test.go`
- `Nukara_Backend/internal/agentx/memory/store_test.go`
- new admin memory handler tests where needed

Add tests for:

- memory category classification
- user-fact admission filtering
- high-risk persona-change routing to pending confirmation
- chat-only promise lifecycle
- current-state continuity across turns
- detail profile response shape
- post-turn model configuration selection and fallback behavior
- admin memory/state inspection extensions

## Frontend tests

User web:

- detail page renders runtime-state card
- detail page renders key memories / recent changes / pending changes
- impression no longer depends on manual regeneration for the main UX

Admin web:

- memory panel shows new kinds/statuses and state/change sections
- post-turn model config can be read and updated

## Risks and Mitigations

### Risk: memory bloat

Mitigation:

- strict candidate filtering
- deduplication/merge rules
- top-k prompt injection only
- archive/expire low-value items

### Risk: persona drift

Mitigation:

- only core persona mutations require confirmation
- visible recent-change log
- clear separation between stable persona and runtime state

### Risk: contradictory living state

Mitigation:

- dedicated runtime-state object
- continuity-first updates
- consistency guardrail in runtime prompt assembly

### Risk: post-turn pipeline cost

Mitigation:

- separate cheaper model configuration
- run heavy summarization on cadence, not every turn
- fallback behavior when dedicated config is absent

### Risk: duplicated admin tooling

Mitigation:

- extend the already-landed Admin memory graph path on `main`
- keep user detail page and admin inspection page clearly separated by audience

## Implementation Notes and Baseline Constraint

Because `main` advanced on 2026-03-08 with `bb0b6bc`, implementation should treat that commit as the baseline for follow-up work. The current brainstorming worktree is one commit behind `main`, so implementation planning should explicitly account for syncing or rebasing onto the latest `main` before code changes begin.

## Approved Design Summary

The approved first-release design is:

- split stable persona, long-term memory, events/promises, and current living state
- keep `memory_items` as the primary extracted-memory store, with richer `kind/status` semantics
- add a dedicated runtime-state record and a persona-change event log
- move impression/iteration generation into the asynchronous post-turn pipeline
- give the post-turn pipeline its own cheaper configurable model, managed in Admin
- redesign the user bot-detail page into a runtime portrait page
- extend the newly landed Admin memory graph tooling to inspect the richer memory/state/change system
- keep promise follow-through inside chat only for now
