package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)
func (m Model) updateFeed(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	var needsContentUpdate bool

	// Ensure viewport is initialized if we have dimensions but Width is 0
	if m.feedViewport.Width == 0 && m.width > 0 {
		// Calculate header height dynamically
		headerHeight := lipgloss.Height(TitleStyle.Render(" MOLTBOOK ") + "  " + HeaderStyle.Render(m.feedTitle)) +
			lipgloss.Height(HelpStyle.Render("j/k: select • ↑/↓: scroll • enter: view • u: upvote • p: profile • f/h: feeds • n: new • r: refresh • q: quit")) +
			2 // For the two newlines after the help text
		m.feedViewport.Width = m.width
		m.feedViewport.Height = m.height - headerHeight
		needsContentUpdate = true
		m.ready = true
	}

	switch msg := msg.(type) {
	case commentsMsg:
		needsContentUpdate = true
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if m.selectedIndex < len(m.posts)-1 {
				m.selectedIndex++
				needsContentUpdate = true
				m.paginationErr = nil // Clear error on movement
				if m.selectedIndex >= len(m.posts)-2 && !m.isPaginating && !m.allPostsLoaded {
					m.isPaginating = true
					cmd = m.loadMoreCmd()
				}
			}
		case "k", "up":
			if m.selectedIndex > 0 {
				m.selectedIndex--
				needsContentUpdate = true
				m.paginationErr = nil // Clear error on movement
			}
		}
	case tea.WindowSizeMsg:
		m.feedViewport.Width = msg.Width
		m.feedViewport.Height = msg.Height - 4
		needsContentUpdate = true
	case spinner.TickMsg:
		if m.isPaginating {
			needsContentUpdate = true
		}
	}

	if needsContentUpdate && m.feedViewport.Width > 0 && m.feedViewport.Height > 0 {
		content, offsets := m.renderFeedContent()
		m.feedViewport.SetContent(content)
		
		// Smart scrolling based on selection
		if m.selectedIndex >= 0 && m.selectedIndex < len(offsets)-1 {
			start := offsets[m.selectedIndex]
			end := offsets[m.selectedIndex+1]
			// Add partial padding (e.g. 1 line) if possible
			end += 1 
			
			// Adjust viewport
			if start < m.feedViewport.YOffset {
				m.feedViewport.YOffset = start
			}
			if end > m.feedViewport.YOffset + m.feedViewport.Height {
				m.feedViewport.YOffset = end - m.feedViewport.Height
			}
			// Priority: Top of the post should be visible if it's larger than viewport
			if start < m.feedViewport.YOffset {
				m.feedViewport.YOffset = start
			}
		}
		
		// Bounds check
		numLines := strings.Count(content, "\n")
		if m.feedViewport.YOffset > numLines-m.feedViewport.Height {
			m.feedViewport.YOffset = numLines - m.feedViewport.Height
		}
		if m.feedViewport.YOffset < 0 {
			m.feedViewport.YOffset = 0
		}
	}

	var vpCmd tea.Cmd
	m.feedViewport, vpCmd = m.feedViewport.Update(msg)
	return m, tea.Batch(cmd, vpCmd)
}

func (m Model) renderFeedContent() (string, []int) {
	var s strings.Builder
	var offsets []int
	currentLine := 0

	if len(m.posts) == 0 {
		s.WriteString("No posts found. Press 'r' to refresh.\n")
		return s.String(), []int{0, 1}
	}

	for i, post := range m.posts {
		offsets = append(offsets, currentLine)
		
		style := PostCardStyle
		if i == m.selectedIndex {
			style = SelectedPostStyle
		}

		title := post.Title
		if title == "" {
			title = "Post"
		}

		content := post.Content
		if len(content) > 100 {
			content = content[:97] + "..."
		}

		upvoteIndicator := ""
		if m.upvotedPosts[post.ID] {
			upvoteIndicator = lipgloss.NewStyle().Foreground(PrimaryColor).Bold(true).Render(" [UPVOTED]")
		}
		meta := fmt.Sprintf("%s · %s · %d 🦞%s", AuthorStyle.Render(post.Author.Name), SubmoltStyle.Render("m/"+post.Submolt.Name), post.Upvotes, upvoteIndicator)
		
		card := style.Width(m.width - 4).Render(
			fmt.Sprintf("%s\n%s\n\n%s", lipgloss.NewStyle().Bold(true).Render(title), content, meta),
		)
		s.WriteString(card + "\n")
		
		// Count lines (newlines + 1)
		lines := strings.Count(card, "\n") + 1
		currentLine += lines
	}
	offsets = append(offsets, currentLine) // Sentinel end position
	
	if m.isPaginating {
		loadingText := lipgloss.NewStyle().Foreground(AccentColor).Render(fmt.Sprintf("\n   %s Loading...", m.spinner.View()))
		s.WriteString(loadingText)
	}
	
	return s.String(), offsets
}

func (m Model) feedView() string {
	var s strings.Builder
	s.WriteString(TitleStyle.Render(" MOLTBOOK ") + "  " + HeaderStyle.Render(m.feedTitle))
	s.WriteString("\n" + HelpStyle.Render("j/k: select • ↑/↓: scroll • enter: view • u: upvote • p: profile • f/h: feeds • n: new • r: refresh • q: quit"))
	s.WriteString("\n\n")
	s.WriteString(m.feedViewport.View())
	
	if m.message != "" {
		s.WriteString("\n" + lipgloss.NewStyle().Foreground(AccentColor).Render("• "+m.message))
	}

	return s.String()
}

func (m Model) upvoteCmd(id string) tea.Cmd {
	return func() tea.Msg {
		err := m.client.UpvotePost(id)
		if err != nil {
			return messageMsg("Upvote failed: " + err.Error())
		}
		return upvoteSuccessMsg(id)
	}
}

func (m Model) followCmd(name string) tea.Cmd {
	return func() tea.Msg {
		err := m.client.Follow(name)
		if err != nil {
			return errMsg{err}
		}
		return messageMsg(fmt.Sprintf("Following %s! 🦞", name))
	}
}
