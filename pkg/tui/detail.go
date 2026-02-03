package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/starkbaknet/moltbook-client/pkg/api"
)

type commentCreatedMsg struct {
	err error
}

func (m Model) updateCreateComment(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if m.isSubmitting {
				return m, nil
			}
			content := m.textInput.Value()
			if content != "" {
				m.isSubmitting = true
				return m, m.createCommentCmd(content)
			}
		case "esc":
			m.state = statePostDetail
			m.textInput.Blur()
			return m, nil
		}
	}
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m Model) createCommentView() string {
	return lipgloss.JoinVertical(
		lipgloss.Left,
		TitleStyle.Render(" ADD COMMENT "),
		"\n"+m.renderPostHeader(),
		"\n",
		m.textInput.View(),
		"\n"+HelpStyle.Render("enter: post • esc: cancel"),
	)
}

func (m Model) createCommentCmd(content string) tea.Cmd {
	return func() tea.Msg {
		if m.selectedPost == nil {
			return commentCreatedMsg{err: fmt.Errorf("no post selected")}
		}
		err := m.client.CreateComment(m.selectedPost.ID, content)
		return commentCreatedMsg{err: err}
	}
}

func (m Model) updatePostDetail(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	var needsContentUpdate bool
	
	// Ensure viewport is initialized/re-initialized
	if (m.viewport.Width == 0 || !m.ready) && m.width > 0 {
		headerHeight := lipgloss.Height(m.renderPostHeader())
		m.viewport.Width = m.width
		m.viewport.Height = m.height - headerHeight - 2
		needsContentUpdate = true
		m.ready = true
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "b":
			m.state = stateFeed
			m.ready = false
			m.commentIndex = 0
			return m, nil
		case "j", "down":
			if m.commentIndex < len(m.comments)-1 {
				m.commentIndex++
				needsContentUpdate = true
				if m.commentIndex >= len(m.comments)-2 && len(m.comments) > 0 && !m.isLoadingComments {
					m.isLoadingComments = true
					cmd = m.loadMoreCommentsCmd()
				}
			}
		case "k", "up":
			if m.commentIndex > 0 {
				m.commentIndex--
				needsContentUpdate = true
			}
		case "l":
			if len(m.comments) > 0 && !m.isLoadingComments {
				m.isLoadingComments = true
				cmd = m.loadMoreCommentsCmd()
			}
		case "c":
			m.state = stateCreateComment
			m.textInput.Focus()
			m.textInput.SetValue("")
			m.textInput.Placeholder = "Write a comment..."
			return m, nil
		case "u":
			if m.selectedPost != nil {
				return m, m.upvoteCmd(m.selectedPost.ID)
			}
		}
	case commentsMsg:
		m.isLoading = false
		m.isLoadingComments = false
		
		if msg.append {
			m.comments = append(m.comments, msg.comments...)
		} else {
			m.comments = msg.comments
		}
		m.err = msg.err
		needsContentUpdate = true
	case tea.WindowSizeMsg:
		headerHeight := lipgloss.Height(m.renderPostHeader())
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - headerHeight - 2
		needsContentUpdate = true
		m.ready = true
	case spinner.TickMsg:
		if m.isLoadingComments {
			needsContentUpdate = true
		}
	}
	
	if needsContentUpdate && m.ready && m.viewport.Height > 0 {
		content, offsets := m.renderDetailContent()
		m.viewport.SetContent(content)
		
		// Smart scrolling for comments
		if m.commentIndex >= 0 && m.commentIndex < len(offsets)-1 {
			start := offsets[m.commentIndex]
			// If we are at the first comment, allow seeing the post header/content above it
			if m.commentIndex == 0 {
				start = 0
			}
			end := offsets[m.commentIndex+1]
			end += 1 // padding
			
			if start < m.viewport.YOffset {
				m.viewport.YOffset = start
			}
			if end > m.viewport.YOffset + m.viewport.Height {
				m.viewport.YOffset = end - m.viewport.Height
			}
			// Priority: Top
			if start < m.viewport.YOffset {
				m.viewport.YOffset = start
			}
		}

		// ULTIMATE SAFETY: Clamp YOffset relative to the actual content lines
		numLines := strings.Count(content, "\n")
		if m.viewport.YOffset > numLines-m.viewport.Height {
			m.viewport.YOffset = numLines - m.viewport.Height
		}
		if m.viewport.YOffset < 0 {
			m.viewport.YOffset = 0
		}
	}
	
	var vpCmd tea.Cmd
	// Only handle viewport updates for non-selection keys to prevent fighting
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "k", "up", "down":
			// We handle these manually for 'Smart Scrolling'
		default:
			m.viewport, vpCmd = m.viewport.Update(msg)
		}
	default:
		m.viewport, vpCmd = m.viewport.Update(msg)
	}
	
	return m, tea.Batch(cmd, vpCmd)
}

func (m Model) renderPostHeader() string {
	if m.selectedPost == nil {
		return ""
	}
	var s strings.Builder
	s.WriteString(TitleStyle.Render(" "+m.selectedPost.Submolt.DisplayName+" ") + "\n\n")
	s.WriteString(lipgloss.NewStyle().Bold(true).Render(m.selectedPost.Title) + "\n")
	upvoteIndicator := ""
	if m.upvotedPosts[m.selectedPost.ID] {
		upvoteIndicator = lipgloss.NewStyle().Foreground(PrimaryColor).Bold(true).Render(" [UPVOTED]")
	}
	s.WriteString(AuthorStyle.Render(m.selectedPost.Author.Name) + " · " + lipgloss.NewStyle().Foreground(GrayColor).Render(fmt.Sprintf("%d Upvotes", m.selectedPost.Upvotes)) + upvoteIndicator + "\n")
	s.WriteString(lipgloss.NewStyle().Foreground(AccentColor).Render("──────────────────────────────────────────"))
	return s.String()
}

func (m Model) renderDetailContent() (string, []int) {
	var s strings.Builder
	var offsets []int
	currentLine := 0

	// Add Post Content First
	if m.selectedPost != nil {
		postTxt := lipgloss.NewStyle().
			Width(m.width - 4).
			Padding(1, 0).
			Render(m.selectedPost.Content)
		s.WriteString(postTxt)
		s.WriteString("\n\n")
		
		currentLine += strings.Count(postTxt, "\n") + 3 // +3 for \n\n and implicit newline
	}

	if len(m.comments) == 0 {
		if m.isLoadingComments {
			s.WriteString("\n" + lipgloss.NewStyle().Foreground(AccentColor).Render(fmt.Sprintf("   %s Loading discussion...", m.spinner.View())) + "\n")
			return s.String(), []int{0, 0}
		}
		s.WriteString(lipgloss.NewStyle().Italic(true).Foreground(GrayColor).Render("No comments yet. Be the first!") + "\n")
		return s.String(), []int{0, 0}
	}

	header := HeaderStyle.Render(fmt.Sprintf("COMMENTS (%d)", len(m.comments))) + "\n"
	s.WriteString(header)
	currentLine += strings.Count(header, "\n") // Header lines
	
	for i, c := range m.comments {
		offsets = append(offsets, currentLine)
		
		// Selection Style
		borderColor := GrayColor
		if i == m.commentIndex {
			borderColor = PrimaryColor
		}
		
		commentBody := fmt.Sprintf("%s\n%s · %d 🦞\n", c.Content, AuthorStyle.Render(c.Author.Name), c.Upvotes)
		style := lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(borderColor).
			PaddingLeft(1).
			Width(m.width - 4)
			
		renderedComment := style.Render(commentBody)
		s.WriteString(renderedComment + "\n")
		
		currentLine += strings.Count(renderedComment, "\n") + 1
	}
	offsets = append(offsets, currentLine) // Sentinel
	
	if m.isLoadingComments {
		s.WriteString(lipgloss.NewStyle().Foreground(AccentColor).Render(fmt.Sprintf("\n   %s Loading more...", m.spinner.View())))
	}
	
	return s.String(), offsets
}

func (m Model) postDetailView() string {
	msg := ""
	if m.message != "" {
		msg = "\n" + lipgloss.NewStyle().Foreground(AccentColor).Render("• "+m.message)
	}
	return fmt.Sprintf("%s\n%s\n%s%s", 
		m.renderPostHeader(),
		m.viewport.View(),
		HelpStyle.Render("esc: back • j/k: select comment • ↑/↓: scroll • u: upvote post • c: comment"),
		msg,
	)
}

type commentsMsg struct {
	comments []api.Comment
	err      error
	append   bool
	postID   string
}

func (m Model) fetchCommentsCmd(postID string) tea.Cmd {
	return func() tea.Msg {
		if m.client == nil {
			return commentsMsg{err: fmt.Errorf("client not initialized")}
		}
		// Use the explicitly captured postID
		comments, err := m.client.GetComments(postID)
		return commentsMsg{comments: comments, err: err, append: false, postID: postID}
	}
}

func (m Model) loadMoreCommentsCmd() tea.Cmd {
	return func() tea.Msg {
		if m.selectedPost == nil {
			return commentsMsg{append: true}
		}
		// Note: The API might not support pagination for comments yet
		// For now, we'll just return empty to avoid errors
		// In a real implementation, you'd pass offset/limit params
		return commentsMsg{comments: []api.Comment{}, err: nil, append: true, postID: m.selectedPost.ID}
	}
}
