package tmux

// Interop with the tmux-attention plugin.
//
// The plugin owns this schema, not orc: it is a released, versioned artifact,
// and tmux-fzf-jump already consumes it. Orc publishes into it so its markers
// reach the same consumers a hook-set marker does.
//
// Two option families are written for every state change:
//
//   - @agent_pane_attention{,_updated_at,_source} — the plugin's authoritative
//     pane-scoped schema. This is what the plugin's CLI reads, what
//     tmux-fzf-jump's attention view lists, and what clear-on-view clears.
//     Orc previously did not write these at all, so an orc-set marker never
//     appeared in the attention picker.
//   - @agent_attention{,_since,_source} — orc's original names, still written
//     so orc's own readers and any existing reporter keep working. The
//     plugin's tab-icon format reads @agent_attention, and tmux resolves it
//     pane → window → session, which is why the glyph already rendered from
//     the pane-scoped value.
//
// Note the timestamp fields are NOT interchangeable: the plugin records
// _updated_at, orc recorded _since. Writing only one of them is what left each
// side able to read the other's state but not its age.
//
// These are plain option writes rather than shelling out to the plugin's CLI on
// purpose. The plugin is an optional dependency — orc's own lifecycle tracking
// must work without it — and raw writes reach every consumer that matters,
// verified against a live server.

// attentionUpdates returns the pane-option writes publishing one attention
// state under both schemas. It is deliberately mechanical: callers decide the
// timestamp and source, including the empty-state cases.
func attentionUpdates(state, timestamp, source string) [][2]string {
	return [][2]string{
		{"@agent_pane_attention", state},
		{"@agent_pane_attention_updated_at", timestamp},
		{"@agent_pane_attention_source", source},
		{"@agent_attention", state},
		{"@agent_attention_since", timestamp},
		{"@agent_attention_source", source},
	}
}
