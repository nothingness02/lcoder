package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lcoder/lcoder/pkg/models"
	"github.com/lcoder/lcoder/pkg/task"
)

// taskSidebarWidth is the fixed outer width (including border) of the task
// sidebar. Below this much terminal width the sidebar is suppressed entirely.
const taskSidebarWidth = 28

// taskSidebarVisible reports whether the sidebar should render: there are tasks,
// the user hasn't hidden it, and the terminal is wide enough to avoid cramping.
func (m *Model) taskSidebarVisible() bool {
	return len(m.tasks) > 0 && !m.taskSidebarHidden && m.width >= 60
}

// mainContentWidth is the width available to the conversation/composer once the
// task sidebar (if visible) has taken its fixed column.
func (m *Model) mainContentWidth() int {
	if m.taskSidebarVisible() {
		return m.width - taskSidebarWidth
	}
	return m.width
}

// applyTaskUpdate parses a todo_write tool's args into the task list. It returns
// true when the list was replaced, false when the args were malformed (the
// previous list is kept).
func (m *Model) applyTaskUpdate(args map[string]any) bool {
	tasks, err := task.Parse(args["todos"])
	if err != nil {
		return false
	}
	m.tasks = tasks
	return true
}

// toggleTaskSidebar flips the user's hide override and re-lays-out.
func (m *Model) toggleTaskSidebar() {
	m.taskSidebarHidden = !m.taskSidebarHidden
	m.updateSizes()
}

// tasksFromMessages rebuilds the task list from history by finding the most
// recent todo_write tool call. Returns nil when none is present.
func tasksFromMessages(msgs []models.AgentMessage) []task.Task {
	for i := len(msgs) - 1; i >= 0; i-- {
		for _, tc := range msgs[i].ToolCalls() {
			if tc.Name == task.ToolName {
				if tasks, err := task.Parse(tc.Arguments["todos"]); err == nil {
					return tasks
				}
			}
		}
	}
	return nil
}

// taskGlyph maps a status to its sidebar marker.
func taskGlyph(s task.Status) string {
	switch s {
	case task.StatusDone:
		return styleSuccess().Render("✓")
	case task.StatusInProgress:
		return styleAccent().Render("▸")
	default:
		return styleDim().Render("○")
	}
}

// renderTaskSidebar draws the fixed-width bordered task panel of the given outer
// height. Each task is one line "glyph text"; a "done/total" footer closes it.
func renderTaskSidebar(tasks []task.Task, height int) string {
	inner := taskSidebarWidth - 2 // left/right border columns
	lines := []string{styleAccent().Render("Tasks"), ""}
	for _, t := range tasks {
		text := truncateCells(t.Text, inner-2, "…") // leave room for "glyph "
		lines = append(lines, taskGlyph(t.Status)+" "+text)
	}
	done, _, _ := task.Counts(tasks)
	lines = append(lines, "", styleDim().Render(fmt.Sprintf("%d/%d 完成", done, len(tasks))))

	boxHeight := height - 2 // top/bottom border rows
	if boxHeight < 1 {
		boxHeight = 1
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorFaint).
		Width(inner).
		Height(boxHeight)
	return box.Render(strings.Join(lines, "\n"))
}
