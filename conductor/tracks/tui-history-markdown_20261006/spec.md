# Specification: TUI Command History & Markdown Rendering Fixes

## Track Overview
Enhance the WhatsADK Command TUI (`cmd/tui`, `internal/tui/app.go`) to support keyboard Up/Down arrow command history navigation and rich Markdown rendering (using Glamour / Lip Gloss) for `help` and Markdown-type command results.

## Key Functional Requirements

1. **Command History Navigation (Up / Down Arrow Keys):**
   - Pressing **Up Arrow** (`up`) in the terminal prompt traverses backwards through executed command history.
   - Pressing **Down Arrow** (`down`) traverses forwards through command history back to the active draft input.
   - Preserves unsubmitted draft input when navigating into history.

2. **Rich Markdown Rendering Viewport:**
   - Integrate Glamour (`github.com/charmbracelet/glamour`) ANSI Markdown renderer for all `ResultTypeMarkdown` command outputs (including `help`, `sql`, `query_contacts`, `jid_to_phone`, etc.).
   - Converts raw Markdown symbols (`###`, `**`, `` ` ``) into styled ANSI titles, bold text, formatted JSON/code blocks, and list items inside the viewport.

3. **Input Prompt & Status Bar Layout Cleanup:**
   - Clean up input bar rendering to remove stray text/arrow clutter on the prompt line.
