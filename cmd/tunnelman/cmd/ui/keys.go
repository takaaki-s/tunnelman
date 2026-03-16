package ui

const (
	keyStart    = "s"
	keyStop     = "S"
	keyAdd      = "a"
	keyEdit     = "e"
	keyDelete   = "d"
	keyRefresh  = "r"
	keyHelp     = "?"
	keyQuit     = "q"
	keyCtrlC    = "ctrl+c"
	keyStartAll = "ctrl+s"
	keyStopAll  = "ctrl+x"
)

// hintText is shown in the status bar when no status message is set.
const hintText = "tab/shift+tab=profile  s=start  S=stop  ^S=start-all  ^X=stop-all  a=add  e=edit  d=del  r=reload  ?=help  q=quit"

// formHintText is shown at the bottom of the add/edit form.
const formHintText = "  Tab=next/wrap  Shift+Tab=prev  Enter=next/confirm  Esc=cancel"
