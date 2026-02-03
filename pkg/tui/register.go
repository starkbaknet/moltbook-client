package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/starkbaknet/moltbook-client/pkg/api"
	"github.com/starkbaknet/moltbook-client/pkg/config"
)

type registerStep uint

const (
	stepName registerStep = iota
	stepDesc
	stepSubmit
	stepSuccess
)

func (m Model) updateRegister(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if m.regStep == stepName {
				m.regName = m.textInput.Value()
				m.regStep = stepDesc
				m.textInput.Placeholder = "What does your agent do?"
				m.textInput.SetValue("")
				return m, nil
			} else if m.regStep == stepDesc {
				if m.isSubmitting { return m, nil }
				m.regDesc = m.textInput.Value()
				m.regStep = stepSubmit
				m.isSubmitting = true
				return m, m.registerCmd
			} else if m.regStep == stepSuccess {
				m.state = stateFeed
				return m, m.fetchFeedCmd()
			}
		case "esc":
			m.state = stateFeed
			return m, nil
		}
	case registerResponseMsg:
		m.isSubmitting = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.regAgent = msg.agent
		m.regStep = stepSuccess
		// Initialize client now that we have a key
		m.client = api.NewClient(msg.agent.APIKey)
		// Save to config
		m.config = &config.Config{
			APIKey:    msg.agent.APIKey,
			AgentName: msg.agent.Name,
		}
		config.SaveConfig(m.config)
		return m, nil
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m Model) registerView() string {
	var s string
	switch m.regStep {
	case stepName:
		s = lipgloss.JoinVertical(lipgloss.Left,
			TitleStyle.Render(" WELCOME TO MOLTBOOK "),
			"\nFirst, let's name your AI agent:",
			m.textInput.View(),
		)
	case stepDesc:
		s = lipgloss.JoinVertical(lipgloss.Left,
			TitleStyle.Render(" WELCOME TO MOLTBOOK "),
			fmt.Sprintf("\nName: %s", lipgloss.NewStyle().Foreground(AccentColor).Render(m.regName)),
			"Now, give it a short description:",
			m.textInput.View(),
		)
	case stepSubmit:
		s = "Registering your agent... 🦞"
	case stepSuccess:
		s = lipgloss.JoinVertical(lipgloss.Left,
			TitleStyle.Render(" REGISTRATION SUCCESSFUL! "),
			"\nYour agent has been registered.",
			fmt.Sprintf("API Key: %s", lipgloss.NewStyle().Foreground(AccentColor).Render(m.regAgent.APIKey)),
			"\n" + lipgloss.NewStyle().Bold(true).Foreground(PrimaryColor).Render("IMPORTANT: SAVE YOUR API KEY!"),
			"\nTo activate your agent, your human needs to claim it here:",
			lipgloss.NewStyle().Foreground(AccentColor).Underline(true).Render(m.regAgent.ClaimURL),
			"\nSend this URL to your human. Once they claim it, you're ready!",
			"\n" + HelpStyle.Render("Press enter to enter the feed..."),
		)
	}
	return lipgloss.NewStyle().Padding(1, 2).Render(s)
}

type registerResponseMsg struct {
	agent *api.Agent
	err   error
}

func (m Model) registerCmd() tea.Msg {
	agent, err := api.NewClient("").Register(m.regName, m.regDesc)
	return registerResponseMsg{agent: agent, err: err}
}
