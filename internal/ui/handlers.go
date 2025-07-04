package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"codeberg.org/snonux/tasksamurai/internal/task"
)

// handleTextInput provides generic text input handling for all input modes
func (m *Model) handleTextInput(msg tea.KeyMsg, input *textinput.Model, onEnter func(string) error, onExit func()) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		value := input.Value()
		if err := onEnter(value); err != nil {
			m.statusMsg = fmt.Sprintf("Error: %v", err)
			cmd := tea.Tick(2*time.Second, func(time.Time) tea.Msg {
				return struct{ clearStatus bool }{true}
			})
			return m, cmd
		}
		input.Blur()
		onExit()
		m.updateTableHeight()
		return m, nil
	case tea.KeyEsc:
		input.Blur()
		onExit()
		m.updateTableHeight()
		return m, nil
	}
	var cmd tea.Cmd
	*input, cmd = input.Update(msg)
	return m, cmd
}

// handleAnnotationMode handles keyboard input when in annotation mode
func (m *Model) handleAnnotationMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	onEnter := func(value string) error {
		// Annotation can be empty when replacing (to remove all)
		if !m.replaceAnnotations && strings.TrimSpace(value) == "" {
			return fmt.Errorf("annotation cannot be empty")
		}
		
		if m.replaceAnnotations {
			if err := task.ReplaceAnnotations(m.annotateID, value); err != nil {
				return err
			}
			m.replaceAnnotations = false
		} else {
			if err := task.Annotate(m.annotateID, value); err != nil {
				return err
			}
		}
		m.reload()
		return nil
	}
	
	onExit := func() {
		m.annotating = false
		m.replaceAnnotations = false
	}
	
	model, cmd := m.handleTextInput(msg, &m.annotateInput, onEnter, onExit)
	if msg.Type == tea.KeyEnter && m.annotateInput.Value() != "" {
		// Start blink after successful annotation
		return model, m.startBlink(m.annotateID, false)
	}
	return model, cmd
}

// handleDescriptionMode handles keyboard input when editing description
func (m *Model) handleDescriptionMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	onEnter := func(value string) error {
		if err := validateDescription(value); err != nil {
			return err
		}
		if err := task.SetDescription(m.descID, value); err != nil {
			return err
		}
		m.reload()
		return nil
	}
	
	onExit := func() {
		m.descEditing = false
	}
	
	model, cmd := m.handleTextInput(msg, &m.descInput, onEnter, onExit)
	if msg.Type == tea.KeyEnter {
		return model, m.startBlink(m.descID, false)
	}
	return model, cmd
}

// handleTagsMode handles keyboard input when editing tags
func (m *Model) handleTagsMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	onEnter := func(value string) error {
		words := strings.Fields(value)
		var adds, removes []string
		for _, w := range words {
			if strings.HasPrefix(w, "-") {
				if len(w) > 1 {
					tagName := w[1:]
					if err := validateTagName(tagName); err != nil {
						return fmt.Errorf("remove tag '%s': %w", tagName, err)
					}
					removes = append(removes, tagName)
				}
			} else {
				if strings.HasPrefix(w, "+") {
					w = w[1:]
				}
				if w != "" {
					if err := validateTagName(w); err != nil {
						return fmt.Errorf("add tag '%s': %w", w, err)
					}
					adds = append(adds, w)
				}
			}
		}
		if len(adds) > 0 {
			if err := task.AddTags(m.tagsID, adds); err != nil {
				return err
			}
		}
		if len(removes) > 0 {
			if err := task.RemoveTags(m.tagsID, removes); err != nil {
				return err
			}
		}
		m.reload()
		return nil
	}
	
	onExit := func() {
		m.tagsEditing = false
	}
	
	model, cmd := m.handleTextInput(msg, &m.tagsInput, onEnter, onExit)
	if msg.Type == tea.KeyEnter {
		if m.showTaskDetail {
			// In detail view, blink the tags field
			return model, m.startDetailBlink(4) // Tags is field index 4
		}
		return model, m.startBlink(m.tagsID, false)
	}
	return model, cmd
}

// handleDueEditMode handles due date editing
func (m *Model) handleDueEditMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		if err := task.SetDueDate(m.dueID, m.dueDate.Format("2006-01-02")); err != nil {
			m.statusMsg = fmt.Sprintf("Error: %v", err)
			cmd := tea.Tick(2*time.Second, func(time.Time) tea.Msg {
				return struct{ clearStatus bool }{true}
			})
			return m, cmd
		}
		m.dueEditing = false
		m.reload()
		var cmd tea.Cmd
		if m.showTaskDetail {
			// In detail view, blink the due field
			cmd = m.startDetailBlink(5) // Due is field index 5
		} else {
			cmd = m.startBlink(m.dueID, false)
		}
		m.updateTableHeight()
		return m, cmd
	case tea.KeyEsc:
		m.dueEditing = false
		m.updateTableHeight()
		return m, nil
	}
	
	switch msg.String() {
	case "h", "left":
		m.dueDate = m.dueDate.AddDate(0, 0, -1)
	case "l", "right":
		m.dueDate = m.dueDate.AddDate(0, 0, 1)
	case "k", "up":
		m.dueDate = m.dueDate.AddDate(0, 0, -7)
	case "j", "down":
		m.dueDate = m.dueDate.AddDate(0, 0, 7)
	}
	return m, nil
}

// handleRecurrenceMode handles recurrence editing
func (m *Model) handleRecurrenceMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	onEnter := func(value string) error {
		if err := validateRecurrence(value); err != nil {
			return err
		}
		if err := task.SetRecurrence(m.recurID, value); err != nil {
			return err
		}
		m.reload()
		return nil
	}
	
	onExit := func() {
		m.recurEditing = false
	}
	
	model, cmd := m.handleTextInput(msg, &m.recurInput, onEnter, onExit)
	if msg.Type == tea.KeyEnter {
		if m.showTaskDetail {
			// In detail view, blink the recurrence field (dynamic index)
			// Need to calculate the index based on whether recurrence field exists
			fieldIndex := 8 // Base index for recurrence
			if m.currentTaskDetail != nil && m.currentTaskDetail.Recur != "" {
				return model, m.startDetailBlink(fieldIndex)
			}
		}
		return model, m.startBlink(m.recurID, false)
	}
	return model, cmd
}

// handleProjectMode handles project editing
func (m *Model) handleProjectMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	onEnter := func(value string) error {
		return task.SetProject(m.projID, value)
	}
	
	onExit := func() {
		m.projEditing = false
		m.reload()
	}
	
	model, cmd := m.handleTextInput(msg, &m.projInput, onEnter, onExit)
	if msg.Type == tea.KeyEnter {
		if m.showTaskDetail {
			// In detail view, blink the project field
			return model, m.startDetailBlink(fieldProject) // Project field index in detail view
		}
		return model, m.startBlink(m.projID, false)
	}
	return model, cmd
}

// handlePriorityMode handles priority selection
func (m *Model) handlePriorityMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		priority := priorityOptions[m.priorityIndex]
		if err := validatePriority(priority); err != nil {
			m.statusMsg = fmt.Sprintf("Error: %v", err)
			cmd := tea.Tick(2*time.Second, func(time.Time) tea.Msg {
				return struct{ clearStatus bool }{true}
			})
			return m, cmd
		}
		if err := task.SetPriority(m.priorityID, priority); err != nil {
			m.statusMsg = fmt.Sprintf("Error: %v", err)
			cmd := tea.Tick(2*time.Second, func(time.Time) tea.Msg {
				return struct{ clearStatus bool }{true}
			})
			return m, cmd
		}
		m.prioritySelecting = false
		m.reload()
		var cmd tea.Cmd
		if m.showTaskDetail {
			// In detail view, blink the priority field
			cmd = m.startDetailBlink(3) // Priority is field index 3
		} else {
			cmd = m.startBlink(m.priorityID, false)
		}
		m.updateTableHeight()
		return m, cmd
	case tea.KeyEsc:
		m.prioritySelecting = false
		m.updateTableHeight()
		return m, nil
	}
	
	switch msg.String() {
	case "h", "left":
		m.priorityIndex = (m.priorityIndex + len(priorityOptions) - 1) % len(priorityOptions)
	case "l", "right":
		m.priorityIndex = (m.priorityIndex + 1) % len(priorityOptions)
	}
	return m, nil
}

// handleFilterMode handles filter editing
func (m *Model) handleFilterMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	onEnter := func(value string) error {
		m.filters = strings.Fields(value)
		m.reload()
		return nil
	}
	
	onExit := func() {
		m.filterEditing = false
	}
	
	return m.handleTextInput(msg, &m.filterInput, onEnter, onExit)
}

// handleAddTaskMode handles adding a new task
func (m *Model) handleAddTaskMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		oldIDs := make(map[int]struct{})
		for _, tsk := range m.tasks {
			oldIDs[tsk.ID] = struct{}{}
		}
		
		if err := task.AddLine(m.addInput.Value()); err != nil {
			m.statusMsg = fmt.Sprintf("Error: %v", err)
			cmd := tea.Tick(2*time.Second, func(time.Time) tea.Msg {
				return struct{ clearStatus bool }{true}
			})
			return m, cmd
		}
		
		m.addingTask = false
		m.addInput.Blur()
		m.reload()
		
		// Find the newly added task
		var newID int
		row := -1
		for i, tsk := range m.tasks {
			if _, ok := oldIDs[tsk.ID]; !ok {
				newID = tsk.ID
				row = i
				break
			}
		}
		
		m.updateTableHeight()
		if row >= 0 {
			prevRow := m.tbl.Cursor()
			prevCol := m.tbl.ColumnCursor()
			m.tbl.SetCursor(row)
			m.tbl.SetColumnCursor(7) // Description column
			m.updateSelectionHighlight(prevRow, m.tbl.Cursor(), prevCol, m.tbl.ColumnCursor())
			return m, m.startBlink(newID, false)
		}
		return m, nil
		
	case tea.KeyEsc:
		m.addingTask = false
		m.addInput.Blur()
		m.updateTableHeight()
		return m, nil
	}
	
	var cmd tea.Cmd
	m.addInput, cmd = m.addInput.Update(msg)
	return m, cmd
}

// handleSearchMode handles search input
func (m *Model) handleSearchMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		pattern := m.searchInput.Value()
		if pattern != "" {
			// Check cache first
			if cached, ok := searchRegexCache[pattern]; ok {
				m.searchRegex = cached
			} else {
				// Compile and cache if not found
				re, err := compileAndCacheRegex(pattern)
				if err == nil {
					m.searchRegex = re
				} else {
					m.searchRegex = nil
					m.statusMsg = fmt.Sprintf("Invalid regex: %v", err)
				}
			}
		} else {
			m.searchRegex = nil
		}
		m.searching = false
		m.searchInput.Blur()
		m.reload()
		m.updateTableHeight()
		
		if len(m.searchMatches) > 0 {
			match := m.searchMatches[m.searchIndex]
			prevRow := m.tbl.Cursor()
			prevCol := m.tbl.ColumnCursor()
			m.tbl.SetCursor(match.row)
			m.tbl.SetColumnCursor(match.col)
			m.updateSelectionHighlight(prevRow, m.tbl.Cursor(), prevCol, m.tbl.ColumnCursor())
		}
		return m, nil
		
	case tea.KeyEsc:
		m.searching = false
		m.searchInput.Blur()
		m.updateTableHeight()
		return m, nil
	}
	
	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	return m, cmd
}

// handleHelpSearchMode handles search input in help mode
func (m *Model) handleHelpSearchMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		pattern := m.helpSearchInput.Value()
		if pattern != "" {
			// Check cache first
			if cached, ok := searchRegexCache[pattern]; ok {
				m.helpSearchRegex = cached
			} else {
				// Compile and cache if not found
				re, err := compileAndCacheRegex(pattern)
				if err == nil {
					m.helpSearchRegex = re
				} else {
					m.helpSearchRegex = nil
					m.statusMsg = fmt.Sprintf("Invalid regex: %v", err)
				}
			}
		} else {
			m.helpSearchRegex = nil
		}
		m.helpSearching = false
		m.helpSearchInput.Blur()
		
		// Find matching help lines
		m.helpSearchMatches = nil
		if m.helpSearchRegex != nil {
			helpLines := m.getHelpLines()
			for i, line := range helpLines {
				if m.helpSearchRegex.MatchString(line) {
					m.helpSearchMatches = append(m.helpSearchMatches, i)
				}
			}
			// Set to first match
			if len(m.helpSearchMatches) > 0 {
				m.helpSearchIndex = 0
			}
		}
		return m, nil
		
	case tea.KeyEsc:
		m.helpSearching = false
		m.helpSearchInput.Blur()
		return m, nil
	}
	
	var cmd tea.Cmd
	m.helpSearchInput, cmd = m.helpSearchInput.Update(msg)
	return m, cmd
}

// handleBlinkingState handles input when a task is blinking
func (m *Model) handleBlinkingState(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(tea.KeyMsg); ok {
		// Only allow navigation while blinking
		prevRow := m.tbl.Cursor()
		prevCol := m.tbl.ColumnCursor()
		var cmd tea.Cmd
		m.tbl, cmd = m.tbl.Update(msg)
		if prevRow != m.tbl.Cursor() || prevCol != m.tbl.ColumnCursor() {
			m.updateSelectionHighlight(prevRow, m.tbl.Cursor(), prevCol, m.tbl.ColumnCursor())
		}
		return m, cmd
	}
	return m, nil
}

// handleEditingModes checks if we're in any editing mode and handles it
func (m *Model) handleEditingModes(msg tea.KeyMsg) (handled bool, model tea.Model, cmd tea.Cmd) {
	switch {
	case m.annotating:
		model, cmd = m.handleAnnotationMode(msg)
		return true, model, cmd
	case m.descEditing:
		model, cmd = m.handleDescriptionMode(msg)
		return true, model, cmd
	case m.tagsEditing:
		model, cmd = m.handleTagsMode(msg)
		return true, model, cmd
	case m.dueEditing:
		model, cmd = m.handleDueEditMode(msg)
		return true, model, cmd
	case m.recurEditing:
		model, cmd = m.handleRecurrenceMode(msg)
		return true, model, cmd
	case m.projEditing:
		model, cmd = m.handleProjectMode(msg)
		return true, model, cmd
	case m.prioritySelecting:
		model, cmd = m.handlePriorityMode(msg)
		return true, model, cmd
	case m.filterEditing:
		model, cmd = m.handleFilterMode(msg)
		return true, model, cmd
	case m.addingTask:
		model, cmd = m.handleAddTaskMode(msg)
		return true, model, cmd
	case m.searching:
		model, cmd = m.handleSearchMode(msg)
		return true, model, cmd
	case m.helpSearching:
		model, cmd = m.handleHelpSearchMode(msg)
		return true, model, cmd
	}
	return false, m, nil
}

// getSelectedTaskID extracts the task ID from the selected row
func (m *Model) getSelectedTaskID() (int, error) {
	row := m.tbl.SelectedRow()
	if row == nil {
		return 0, fmt.Errorf("no row selected")
	}
	idStr := ansi.Strip(row[1])
	return strconv.Atoi(idStr)
}

// getTaskAtCursor returns the task at the current cursor position
func (m *Model) getTaskAtCursor() *task.Task {
	cursor := m.tbl.Cursor()
	if cursor < 0 || cursor >= len(m.tasks) {
		return nil
	}
	return &m.tasks[cursor]
}

// handleTaskDetailMode handles keyboard input in task detail view
func (m *Model) handleTaskDetailMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.detailSearching {
		var cmd tea.Cmd
		switch msg.Type {
		case tea.KeyEnter:
			pattern := m.detailSearchInput.Value()
			if pattern != "" {
				re, err := compileAndCacheRegex(pattern)
				if err == nil {
					m.detailSearchRegex = re
				} else {
					m.detailSearchRegex = nil
					m.statusMsg = fmt.Sprintf("Invalid regex: %v", err)
				}
			} else {
				m.detailSearchRegex = nil
			}
			m.detailSearching = false
			m.detailSearchInput.Blur()
			return m, nil
		case tea.KeyEsc, tea.KeyCtrlC:
			m.detailSearching = false
			m.detailSearchInput.Blur()
			return m, nil
		default:
			m.detailSearchInput, cmd = m.detailSearchInput.Update(msg)
			return m, cmd
		}
	}
	
	// Normal task detail view mode
	switch msg.String() {
	case "q", "esc":
		return m.handleQuitOrEscape()
	case "/", "?":
		m.detailSearching = true
		m.detailSearchInput.SetValue("")
		m.detailSearchInput.Focus()
		return m, nil
	case "n":
		// Next search match - not implemented yet but could be added
		return m, nil
	case "N":
		// Previous search match - not implemented yet but could be added
		return m, nil
	case "up", "k":
		if m.detailFieldIndex > 0 {
			m.detailFieldIndex--
		}
		return m, nil
	case "down", "j":
		maxFields := m.getDetailFieldCount()
		if m.detailFieldIndex < maxFields-1 {
			m.detailFieldIndex++
		}
		return m, nil
	case "g", "home":
		m.detailFieldIndex = 0
		return m, nil
	case "G", "end":
		m.detailFieldIndex = m.getDetailFieldCount() - 1
		return m, nil
	case "i", "enter":
		// Check if current field is editable
		return m.handleDetailFieldEdit()
	}
	
	return m, nil
}

// handleDetailFieldEdit starts editing for the current field in detail view
func (m *Model) handleDetailFieldEdit() (tea.Model, tea.Cmd) {
	if m.currentTaskDetail == nil {
		return m, nil
	}
	
	id := m.currentTaskDetail.ID
	
	// Map detail field index to editable fields
	// fieldPriority = 3, fieldTags = 4, fieldDue = 5, fieldStart = 6, fieldProject = 7, fieldRecur = 9 or 10 (depending on if fields exist)
	
	// Count fields up to current position to handle dynamic fields
	fieldPos := 0
	
	// ID, UUID, Status (0-2)
	if m.detailFieldIndex <= 2 {
		return m, nil // Not editable
	}
	fieldPos = 3
	
	// Priority (3)
	if m.detailFieldIndex == fieldPos {
		m.clearEditingModes()
		m.priorityID = id
		m.prioritySelecting = true
		
		// Set current priority index
		switch m.currentTaskDetail.Priority {
		case "H":
			m.priorityIndex = 0
		case "M":
			m.priorityIndex = 1
		case "L":
			m.priorityIndex = 2
		default:
			m.priorityIndex = 3
		}
		m.updateTableHeight()
		return m, nil
	}
	fieldPos++
	
	// Tags (4)
	if m.detailFieldIndex == fieldPos {
		m.clearEditingModes()
		m.tagsID = id
		m.tagsEditing = true
		m.tagsInput.SetValue("")
		m.tagsInput.Focus()
		m.updateTableHeight()
		return m, nil
	}
	fieldPos++
	
	// Due (5)
	if m.detailFieldIndex == fieldPos {
		m.dueID = id
		if m.currentTaskDetail.Due != "" {
			if ts, err := parseTaskDate(m.currentTaskDetail.Due); err == nil {
				m.dueDate = ts
			} else {
				m.dueDate = time.Now()
			}
		} else {
			m.dueDate = time.Now()
		}
		m.clearEditingModes()
		m.dueEditing = true
		m.updateTableHeight()
		return m, nil
	}
	fieldPos++
	
	// Start (6)
	if m.detailFieldIndex == fieldPos {
		// Start date is not editable in the original code, only toggled via 's' key
		return m, nil
	}
	fieldPos++
	
	// Project (7)
	if m.detailFieldIndex == fieldPos {
		m.clearEditingModes()
		m.projID = id
		m.projEditing = true
		if m.currentTaskDetail.Project != "" {
			m.projInput.SetValue(m.currentTaskDetail.Project)
		} else {
			m.projInput.SetValue("")
		}
		m.projInput.Focus()
		m.updateTableHeight()
		return m, nil
	}
	fieldPos++
	
	// Entry (8)
	if m.detailFieldIndex == fieldPos {
		return m, nil // Not editable
	}
	fieldPos++
	
	// Recurrence (9) - only if it exists
	if m.currentTaskDetail.Recur != "" {
		if m.detailFieldIndex == fieldPos {
			m.clearEditingModes()
			m.recurID = id
			m.recurEditing = true
			m.recurInput.SetValue(m.currentTaskDetail.Recur)
			m.recurInput.Focus()
			m.updateTableHeight()
			return m, nil
		}
		fieldPos++
	}
	
	// Description (10 or 11 depending on recurrence)
	if m.detailFieldIndex == fieldPos {
		// Launch external editor for description
		m.detailDescEditing = true
		desc := ""
		if m.currentTaskDetail != nil {
			desc = m.currentTaskDetail.Description
		}
		return m, editDescriptionCmd(desc)
	}
	
	// Annotations are not editable in detail view
	return m, nil
}