// Package llm is the in-editor agent runtime: the turn loop (Agent.RunTurn),
// the tools the model may call, the per-session workspace, and the
// DecisionWaiter that pauses a turn until the candidate approves, modifies,
// or rejects a proposed tool call.
//
// Model providers implement LLMStreamClient in subpackages (llm/gemini).
// The HTTP surface is package agent; state changes are persisted and fanned
// out through package events.
package llm
