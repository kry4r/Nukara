# Temporal Memory Graph Design

**Date:** 2026-03-08
**Status:** Approved for planning
**Worktree:** `/Users/nidhogg/code/Nukara/.worktrees/issue-15-memory-persona`
**Supersedes:** `docs/plans/2026-03-08-issue-15-memory-persona-design.md` for the long-term memory architecture

## Summary

This design replaces the current vector-first memory recall pipeline with a single-database temporal memory graph built for anthropomorphic chat behavior. The goal is not merely to retrieve semantically similar facts, but to simulate a human-like recall chain: a conversational cue triggers a few anchors, anchors activate nearby episodes and open loops, time orders the recalled material, and the result is compressed into a very small set of memory cards that can shape the response without bloating the prompt.

The system keeps the existing product responsibilities intact:

- fast chat replies
- compact/session management
- admin graph visualization and explainability
- persona continuity and self-cognition evolution
- low token usage

The main architectural move is to unify long-term memory, temporal relationships, compact summaries, and recall traces into one authoritative storage model backed by Postgres and `pgvector`, instead of splitting truth across `memory_items`, Qdrant, and a topic-only Neo4j graph.

## Goals

- Make memory recall behave like human associative recall rather than keyword or vector lookup.
- Preserve fast chat latency by keeping the online memory path deterministic and cheap.
- Reduce prompt memory footprint by assembling a fixed-budget set of short memory cards rather than dumping raw memory lines.
- Support evolving self-cognition, habits, promises, and state continuity without flattening them into static persona fields.
- Keep admin visibility strong by exposing graph structure, activation traces, self-evolution, and compact/session relationships.
- Support migration from the current memory/session/runtime-state data without requiring a hard cutover on day one.

## Non-Goals

- Building a full world simulator or calendar engine.
- Keeping the current Qdrant + Neo4j topic graph as the primary runtime memory path.
- Updating stable persona fields directly from every chat turn.
- Letting the online reply path depend on expensive summarization calls.

## Core Principle

The memory system should mimic how a person recalls things in conversation:

1. A cue appears in the current exchange.
2. A few relevant anchors come to mind.
3. Related episodes and unresolved matters light up around them.
4. Time orders the recalled material.
5. The mind does not replay the entire archive; it compresses the result into a few salient mental fragments.

Therefore the runtime should not feed the model a flat top-k retrieval list. It should feed a small set of structured memory cards generated from an associative activation process.

## Why the Current Architecture Falls Short

Current code paths already use memory during chat, but the behavior remains retrieval-oriented rather than human-like:

- `Nukara_Backend/internal/api/runtime_context.go` uses `memoryRecall.Build(...)` for main chat memory context.
- `Nukara_Backend/internal/agentx/memory/recall.go` is vector-first and only optionally expands into topic suggestions.
- Topic expansion today produces pseudo-memories such as `相关主题：xxx` instead of traversing actual event or state chains.
- Qdrant writes `occurred_at`, but search results do not read it back, and runtime recall rewrites `OccurredAt` to `time.Now()`, which destroys chronological recall quality.
- The current graph only models topic co-occurrence, not episode-to-episode, habit, self-model, or open-loop relationships.

This is sufficient for a semantic lookup feature, but not for anthropomorphic continuity.

## Research Basis

### Local reference: `Nukara_Memory`

`/Users/nidhogg/Desktop/Nukara_Memory` is the strongest local reference for the target interaction pattern. Its retrieval chain is explicitly:

- anchor phase
- timeline phase
- lateral context enrichment
- spreading activation for more distant associations

That shape is much closer to human recall than a top-k vector search list.

### External references

- **Microsoft GraphRAG** contributes context-budgeted graph retrieval and graph-aware context assembly for fixed windows.
- **Graphiti / Zep** contributes temporally-aware graph modeling, incremental updates, and history-sensitive retrieval.
- **Mem0** contributes selective salience extraction and low-token memory use.
- **LightMem** contributes a cognition-inspired split between cheap online use and heavier offline consolidation.

The recommended Nukara design combines these ideas, but optimizes for conversational anthropomorphism rather than document QA.

## Storage Architecture

Use a single authoritative storage layer:

- **Postgres** for relational truth and transactionality
- **`pgvector`** for seed retrieval embeddings
- **JSONB** for structured node payloads
- **GIN / HNSW indexes** for keyword and vector access

This replaces the current need to coordinate a local store, Qdrant, and a separate graph backend for runtime memory.

## Data Model

### `memory_nodes`

Long-term graph nodes.

Suggested columns:

- `id`
- `user_id`
- `bot_id`
- `session_id` nullable
- `node_type`
- `title`
- `summary`
- `body_json`
- `salience`
- `affect_weight`
- `confidence`
- `stability`
- `status`
- `occurred_at`
- `observed_at`
- `valid_from`
- `valid_to`
- `last_accessed_at`
- `source_turn_id`
- `created_at`
- `updated_at`

### `memory_edges`

Typed temporal and semantic relationships.

Suggested columns:

- `source_id`
- `target_id`
- `edge_type`
- `weight`
- `evidence_count`
- `status`
- `created_at`
- `updated_at`

### `memory_embeddings`

Separate vector table keyed by `node_id`.

### `memory_cards`

Materialized prompt-facing cards for fast online use.

Suggested columns:

- `id`
- `user_id`
- `bot_id`
- `card_type`
- `text`
- `backing_node_ids`
- `freshness_score`
- `created_at`
- `updated_at`

### `activation_traces`

Stores why a reply recalled what it did.

Suggested columns:

- `id`
- `user_id`
- `bot_id`
- `conversation_id`
- `turn_id`
- `cue_json`
- `seed_node_ids`
- `activated_node_ids`
- `selected_card_ids`
- `response_excerpt`
- `created_at`

## Node Types

- `episode`
- `state_snapshot`
- `habit`
- `self_fact`
- `preference`
- `promise`
- `goal`
- `user_fact`
- `relationship`
- `relationship_signal`
- `self_model`
- `session_summary`

## Edge Types

- `belongs_to_session`
- `summarizes`
- `follows`
- `caused_by`
- `related_to`
- `supports`
- `contradicts`
- `fulfills`
- `part_of_routine`
- `about_user`
- `updates_self_model`
- `about`

## Session and Compact Integration

Sessions remain first-class conversation objects.

Compacts remain first-class conversation summaries.

However, each compact is also materialized into a `session_summary` memory node so that long-running conversation history participates in long-term graph recall. This is crucial because it bridges:

- session continuity
- compact-based token reduction
- long-term memory recall
- admin explainability

Recommended structure for each compact / `session_summary`:

- `what_happened`
- `what_changed`
- `what_is_still_open`
- `state_progression`

## Self-Cognition and Persona Evolution

Static persona remains the configuration baseline:

- identity
- personality
- expression style
- life context
- taboos and preferences

But runtime anthropomorphic behavior should not come directly from those fields.

Instead, it should mainly come from:

- latest stable `self_model`
- active `state_snapshot`
- unresolved `promise` / `goal`
- relevant `habit`
- recent `episode`
- current `relationship`

`self_model` is versioned. New versions do not overwrite old ones. They supersede them through `updates_self_model` edges so that admin can display an evolution chain and the system can preserve temporal truth.

## Write Path

Every conversation turn produces asynchronous post-turn work.

### Online-safe writes

Allowed to write immediately:

- `episode`
- `state_snapshot`
- `promise`
- high-confidence low-risk `user_fact`
- temporary `relationship_signal`

Allowed to create lightweight edges immediately:

- `belongs_to_session`
- `follows`
- `about_user`
- `related_to`
- low-weight `supports`

### Offline consolidation

Only asynchronous consolidation can update:

- `habit`
- `preference`
- stable `relationship`
- `self_model`
- `session_summary`

This separation is intentional. A person can instantly remember what just happened, but stable self-understanding forms after repeated evidence.

## Risk Boundary for `user_fact`

Persist a stable `user_fact` only if it is:

- explicitly stated by the user
- low-risk
- likely useful in future interaction

Do not automatically stabilize:

- sexual content
- core identity claims
- highly private information
- deep values or worldview claims

Those can be retained as low-confidence `episode` evidence but should not be promoted to stable user memory without stronger confirmation.

## Online Recall Pipeline

Run for every main chat turn, but keep it extremely cheap.

### 1. Cue Parse

Extract lightweight cues from:

- current user message
- last 2–4 conversation turns

Cue classes:

- people
- places
- time expressions
- emotions
- promises / obligations
- relationship terms
- habit signals
- self-cognition triggers

### 2. Always-On Base Recall

Regardless of query type, include a tiny base layer:

- latest `self_model` card
- current `state_snapshot` card
- unresolved `promise` card
- session working set bridge

This makes the bot feel continuously alive instead of only "remembering" on memory-themed questions.

### 3. Seed Retrieval

Retrieve only 3–6 seeds using a hybrid method:

- vector similarity from `memory_embeddings`
- lexical/entity matches
- unresolved promise hits
- current-state tag matches
- `session_summary` bridge hits

### 4. Graph Activation

Spread activation 1–2 hops from seeds with a bounded node budget.

Node score should combine:

- semantic score
- lexical score
- recency decay
- salience
- open-loop bias
- state match
- relationship proximity
- multi-seed support
- node-type prior

### 5. Chain Assembly

Build 2–3 recall chains rather than returning raw nodes.

Typical chain types:

- recent-life chain
- self-understanding chain
- relationship / open-loop chain

### 6. Card Assembly

Convert the activated chains into a fixed set of short cards.

## Prompt Card Protocol

The model should see a very small, fixed-budget set of cards.

### `self_card`

Latest stable self-cognition.

### `state_card`

Current life-state and mood.

### `user_card`

Only the user facts relevant to the current turn.

### `open_loops_card`

Unresolved promises, goals, and pending relational matters.

### `episode_chain_card`

One or two chains of temporally ordered recalled events.

### `session_bridge_card`

Only when the current question depends heavily on the active or prior compacted session.

## Token Budget

Recommended Chinese prompt budget for memory cards:

- `self_card`: 60–100 chars
- `state_card`: 40–80 chars
- `user_card`: 40–80 chars
- `open_loops_card`: 40–80 chars
- `episode_chain_card`: two cards, each 80–120 chars
- `session_bridge_card`: only when needed

Total target: roughly 300–550 Chinese characters.

This keeps the chat path far cheaper than dumping raw memories or full summaries.

## Time and Forgetting Model

Different node types should decay differently:

- `state_snapshot`: fastest decay
- `episode`: medium decay
- `promise`: almost no decay until resolved
- `habit`: slow decay
- `self_model`: slowest decay, versioned rather than overwritten

This mirrors human recall better than a single recency formula.

## Conflict Handling

Never delete old truth when new truth arrives.

Use time-aware conflict resolution instead:

- preserve the old node
- set `valid_to`
- connect with `contradicts` or `updates_self_model`

This ensures the bot evolves over time instead of having its personality clobbered by the latest turn.

## Working Set

Maintain a lightweight session working set separate from the long-term graph:

- recent important node IDs
- recent unresolved topics
- last state transition

This acts like short-term conversational memory and should be consulted before deeper graph recall.

## Admin Requirements

Admin should support five primary views:

1. `Memory Graph`
2. `Activation Trace`
3. `Self Evolution`
4. `Session / Compact Lens`
5. `Open Loops Board`

The most important new view is `Activation Trace`, which should show:

- cue extraction
- selected seeds
- graph activation path
- chosen cards
- reply excerpt

This makes the system inspectable and debuggable.

## Migration Strategy

### 1. `memory_items -> memory_nodes`

Map existing kinds into node types and carry forward timestamps, status, and importance.

### 2. `BotRuntimeState -> state_snapshot`

Convert existing runtime-state rows into latest state nodes.

### 3. persona change history -> `self_model` chain

Convert recent self-cognition and persona evolution into versioned `self_model` nodes.

### 4. compacts -> `session_summary`

Materialize old compacts into graph nodes and connect them to the episodes and open loops they summarize.

## Rollout Plan

### Phase 1: Shadow Build

- create new schema
- dual-write from post-turn pipeline into the new graph
- build admin read-only graph views from the new tables
- keep old runtime recall as production path

### Phase 2: Shadow Recall Compare

- run new recall alongside old recall
- log activation traces and compare card outputs
- measure latency, token usage, and reply quality

### Phase 3: Read Switch

- main chat reads from the new graph recall path
- old memory path remains as rollback fallback

### Phase 4: Full Cutover

- retire old vector/topic runtime path
- keep migration adapters only as compatibility shims

## Performance Targets

Target memory-stage latency budget:

- seed retrieval: < 20 ms
- graph activation + card assembly: < 30 ms
- total memory recall stage: < 60 ms

These are targets, not guarantees, but the online design is built to keep within this order of magnitude.

## Testing Focus

The implementation should prove:

- the main chat runtime uses memory cards from graph recall
- compact summaries participate in recall via `session_summary`
- `self_model` versions evolve through evidence-backed updates
- promises remain highly recallable until fulfilled
- prompt budget stays bounded
- admin graph and activation trace views reflect the new runtime path

## References

Local reference:

- `/Users/nidhogg/Desktop/Nukara_Memory/README.md`
- `/Users/nidhogg/Desktop/Nukara_Memory/src/store.ts`

External references:

- `https://github.com/microsoft/graphrag`
- `https://microsoft.github.io/graphrag/query/local_search/`
- `https://microsoft.github.io/graphrag/query/drift_search/`
- `https://github.com/getzep/graphiti`
- `https://arxiv.org/abs/2501.13956`
- `https://github.com/mem0ai/mem0`
- `https://arxiv.org/abs/2504.19413`
- `https://github.com/zjunlp/LightMem`
- `https://arxiv.org/abs/2510.18866`
