# TUI

- Esc always goes back one level, in every module and dialog: a form, prompt or confirm closes (discarding unsaved input); a module at its top level closes to home by returning `closeModule` (`app.go`). New modules must do the same, and show Esc in their key bar.
- Enter (and Ctrl+S) in a form saves the whole entry from any field, checklists included, as from a text input; Space toggles a checklist item.
- Home's sidebar (Feeds) stays on the left in every module: a module draws only its panes, right of it (width `sidebarWidth`), and its key bar; `app.go` puts the sidebar beside them. Dialogs and forms open in the right pane, never over the sidebar.
- Navigation is arrow-key first: ↑/↓ move within a list or form, → goes into the selected item (its main content, e.g. a feed's jobs), ← goes back out to where → came from (the sidebar). Every screen reachable by → leaves by ←, unless a text input has focus (there arrows move the cursor). Vim keys (j/k/h/l) mirror arrows where free. Show ←/→ in the key bar.
- F10 and q quit the app from every module, pane, checklist and dialog; q only while no text input has focus (there it types `q`). Quit is handled once in `app.go`: a module reports text focus through `typing()`, never its own q case. Show q in every key bar where it quits.
