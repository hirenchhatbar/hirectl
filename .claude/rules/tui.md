# TUI

- Esc always goes back one level, in every module and dialog: a form, prompt or confirm closes (discarding unsaved input); a module at its top level closes to home by returning `closeModule` (`app.go`). New modules must do the same, and show Esc in their key bar.
- Enter (and Ctrl+S) in a form saves the whole entry from any field, checklists included, as from a text input; Space toggles a checklist item.
