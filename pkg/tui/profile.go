package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/starkbaknet/moltbook-client/pkg/api"
)

func (m Model) updateProfile(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "x": // Shortcut to delete post
			if !m.isSubmitting && len(m.posts) > 0 && m.selectedIndex >= 0 && m.selectedIndex < len(m.posts) {
				m.isSubmitting = true
				return m, m.deletePostCmd(m.posts[m.selectedIndex].ID)
			}
		case "j", "down":
			if m.selectedIndex < len(m.posts)-1 {
				m.selectedIndex++
			}
		case "k", "up":
			if m.selectedIndex > 0 {
				m.selectedIndex--
			}
		}
	}
	return m, nil
}

func (m Model) profileView() string {
	if m.regAgent == nil {
		return "Loading profile..."
	}

	var s strings.Builder
	title := fmt.Sprintf(" PROFILE: %s ", m.regAgent.Name)
	s.WriteString(TitleStyle.Render(title) + "\n\n")
	
	s.WriteString(lipgloss.NewStyle().Bold(true).Render("Description: ") + m.regAgent.Description + "\n")
	s.WriteString(fmt.Sprintf("%s · %s · %s\n", 
		lipgloss.NewStyle().Foreground(AccentColor).Render(fmt.Sprintf("%d Karma", m.regAgent.Karma)),
		fmt.Sprintf("%d Followers", m.regAgent.FollowerCount),
		fmt.Sprintf("%d Following", m.regAgent.FollowingCount),
	))
	
	status := "Claimed ✅"
	if !m.regAgent.IsClaimed {
		status = "Pending Claim ⏳"
	}
	s.WriteString(fmt.Sprintf("Status: %s\n\n", status))

	s.WriteString(HeaderStyle.Render("MY RECENT POSTS") + "\n")
	if len(m.posts) == 0 {
		s.WriteString("You haven't posted anything yet.\n")
	}

	for i, post := range m.posts {
		style := PostCardStyle
		if i == m.selectedIndex {
			style = SelectedPostStyle
		}
		
		title := post.Title
		if title == "" { title = "Post" }
		
		meta := fmt.Sprintf("%d 🦞 · %s", post.Upvotes, post.CreatedAt.Format("2006-01-02"))
		
		card := style.Width(m.width - 4).Render(
			fmt.Sprintf("%s\n%s", lipgloss.NewStyle().Bold(true).Render(title), meta),
		)
		s.WriteString(card + "\n")
	}

	s.WriteString("\n" + HelpStyle.Render("esc: back • enter: view • x: delete post • q: quit"))
	return s.String()
}

type profileMsg struct {
	agent *api.Agent
	posts []api.Post
	err   error
}

func (m Model) fetchMyProfileCmd() tea.Cmd {
	return func() tea.Msg {
		if m.client == nil || m.config == nil {
			return profileMsg{err: fmt.Errorf("not logged in")}
		}
		agent, posts, err := m.client.GetProfile(m.config.AgentName)
		return profileMsg{agent: agent, posts: posts, err: err}
	}
}

func (m Model) deletePostCmd(id string) tea.Cmd {
	return func() tea.Msg {
		err := m.client.DeletePost(id)
		if err != nil {
			return errMsg{err}
		}
		return m.fetchMyProfileCmd()()
	}
}
