package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) updateCreatePost(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			val := m.textInput.Value()
			if val == "" {
				return m, nil
			}

			if m.createStep == 0 {
				// Title Entered
				m.newPostTitle = val
				m.createStep = 1
				m.textInput.SetValue("")
				m.textInput.Placeholder = "Write your content..."
				return m, nil
			} else {
				// Content Entered - Submit
				if m.isSubmitting {
					return m, nil // Prevent duplicate submissions
				}
				content := val
				m.createStep = 0 // Reset for next time
				m.textInput.SetValue("")
				m.isLoading = true
				m.isSubmitting = true
				return m, m.createPostCmd("general", m.newPostTitle, content)
			}
		case "esc":
			m.state = stateFeed
			m.createStep = 0
			m.textInput.Blur()
			return m, nil
		}
	case postCreatedMsg:
		if msg.err != nil {
			m.err = msg.err
			// Stay in create mode if error so they can see it? 
			// Or go back to feed and show error?
			// Current logic: show feed and error will be rendered at top
		}
		m.state = stateFeed
		return m, m.fetchFeedCmd()
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m Model) createPostView() string {
	var stepPrompt string
	var stepInput string

	if m.createStep == 0 {
		stepPrompt = "Title:"
		stepInput = m.textInput.View()
	} else {
		stepPrompt = fmt.Sprintf("Title: %s\nContent:", lipgloss.NewStyle().Bold(true).Render(m.newPostTitle))
		stepInput = m.textInput.View()
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		TitleStyle.Render(" NEW POST "),
		"\nPosting to m/general\n",
		stepPrompt,
		stepInput,
		"\n" + HelpStyle.Render("enter: next/submit • esc: cancel"),
	)
}

type postCreatedMsg struct {
	err error
}

func (m Model) createPostCmd(submolt, title, content string) tea.Cmd {
	return func() tea.Msg {
		err := m.client.CreatePost(submolt, title, content)
		return postCreatedMsg{err: err}
	}
}
