package app

import (
	"github.com/dayanez/Verti/pkg/buffer"
	"github.com/dayanez/Verti/pkg/session"
)

// restoreSession loads the workspace's saved session, if any, and
// replaces the single blank placeholder tab New built with one tab per
// saved, still-readable file, each restored to its saved cursor and
// scroll position, then activates whichever tab was active when the
// session was saved. A missing session, or one whose files have all
// since vanished, just leaves the placeholder tab in place.
func (m *Model) restoreSession(rootDir string) {
	sess, err := session.Load(rootDir)
	if err != nil || len(sess.Tabs) == 0 {
		return
	}

	restored := make([]*openBuffer, 0, len(sess.Tabs))
	for _, t := range sess.Tabs {
		if t.Filename == "" {
			continue
		}
		buf, err := buffer.LoadFile(t.Filename)
		if err != nil {
			continue // gone or unreadable since the session was saved: skip it
		}
		buf.MoveCursorTo(t.CursorOffset)
		restored = append(restored, &openBuffer{
			buf:         buf,
			filename:    t.Filename,
			highlighter: highlighterFor(m.highlighter, t.Filename),
			scrollLine:  t.ScrollLine,
			scrollCol:   t.ScrollCol,
		})
	}
	if len(restored) == 0 {
		return
	}

	m.tabs = restored // the blank placeholder tab New() built is discarded here
	active := sess.ActiveTab
	if active < 0 || active >= len(m.tabs) {
		active = 0
	}
	m.restoreTab(active)
	m.status = "restored session"
}

// saveSession persists which files are open, their cursor and scroll
// positions, and which tab is active, so the next time this workspace is
// opened (with no specific file requested) it picks up where this
// session left off. Untitled/unsaved-to-disk tabs aren't remembered:
// there's no file to reopen them from. Best-effort: an unwritable config
// directory just means nothing gets remembered, not a failure to quit.
func (m *Model) saveSession() {
	m.snapshotActiveTab()

	var sess session.Session
	sess.ActiveTab = -1 // stays -1 if the active tab is untitled and has nothing to restore
	for i, t := range m.tabs {
		if t.filename == "" {
			continue
		}
		sess.Tabs = append(sess.Tabs, session.Tab{
			Filename:     t.filename,
			CursorOffset: t.buf.CursorOffset(),
			ScrollLine:   t.scrollLine,
			ScrollCol:    t.scrollCol,
		})
		if i == m.activeTab {
			sess.ActiveTab = len(sess.Tabs) - 1
		}
	}
	_ = session.Save(m.exp.Root.Path, &sess)
}
