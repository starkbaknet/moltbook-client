package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/starkbaknet/moltbook-client/pkg/api"
	"github.com/starkbaknet/moltbook-client/pkg/config"
)

type sessionState uint

const (
	stateLoading sessionState = iota
	stateFeed
	statePostDetail

	stateCreatePost
	stateCreateComment
	stateRegister
	stateProfile
)

type Model struct {
	state       sessionState
	client      *api.Client
	config      *config.Config
	width, height int

	// Feed components
	posts         []api.Post
	selectedIndex int
	feedTitle     string
	offset        int

	// Detail components
	selectedPost *api.Post
	comments     []api.Comment
	viewport     viewport.Model
	feedViewport viewport.Model
	ready        bool
	commentIndex int

	// Registration/Profile state
	regStep      registerStep
	regName      string
	regDesc      string
	regAgent     *api.Agent

	// Create Post state
	createStep   int // 0: Title, 1: Content
	newPostTitle string

	// Inputs
	textInput   textinput.Model
	spinner     spinner.Model
	isLoading   bool // Full screen loading (initial load, refresh)
	isPaginating bool // Background loading (infinite scroll)
	isLoadingComments bool // Loading comments in detail view
	isSubmitting bool // Prevent duplicate submissions
	
	// Utilities
	help        help.Model
	err         error
	message     string
	paginationErr error
	allPostsLoaded bool
	upvotedPosts   map[string]bool
}

func NewModel() Model {
	ti := textinput.New()
	ti.Placeholder = "Type here..."

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(PrimaryColor)
	
	// Custom keymap for viewports to disable j/k scrolling (we use them for selection)
	vpKeyMap := viewport.DefaultKeyMap()
	vpKeyMap.Up.SetKeys("up")
	vpKeyMap.Down.SetKeys("down")
	vpKeyMap.PageUp.SetKeys("pgup")
	vpKeyMap.PageDown.SetKeys("pgdown")

	fv := viewport.New(0, 0)
	fv.KeyMap = vpKeyMap
	
	dv := viewport.New(0, 0)
	dv.KeyMap = vpKeyMap

	return Model{
		state:        stateLoading,
		textInput:    ti,
		spinner:      s,
		isLoading:    true,
		help:         help.New(),
		feedTitle:    "HOT FEED",
		feedViewport: fv,
		viewport:     dv,
		upvotedPosts: make(map[string]bool),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.loadConfigCmd,
		textinput.Blink,
		m.spinner.Tick,
	)
}

type messageMsg string
type upvoteSuccessMsg string
type configLoadedMsg struct {
	config *config.Config
	client *api.Client
}

func (m Model) loadConfigCmd() tea.Msg {
	cfg, err := config.LoadConfig()
	if err != nil {
		return stateRegister
	}
	return configLoadedMsg{
		config: cfg,
		client: api.NewClient(cfg.APIKey),
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		if m.err != nil {
			switch msg.String() {
			case "q":
				return m, tea.Quit
			case "r":
				m.err = nil
				m.isLoading = true
				switch m.state {
				case stateFeed:
					if m.feedTitle == "HOT FEED" {
						return m, m.fetchFeedCmd()
					}
					return m, m.fetchPersonalizedFeedCmd()
				case statePostDetail:
					if m.selectedPost != nil {
						return m, m.fetchCommentsCmd(m.selectedPost.ID)
					}
					return m, nil
				case stateProfile:
					return m, m.fetchMyProfileCmd()
				default:
					m.state = stateFeed
					return m, m.fetchFeedCmd()
				}
			}
			// Allow navigation keys to bypass error state
		}

		switch keypath := msg.String(); keypath {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			if m.state == stateFeed || m.state == statePostDetail || m.state == stateProfile {
				return m, tea.Quit
			}
		case "esc":
			m.err = nil
			if m.state != stateFeed {
				m.state = stateFeed
				m.textInput.Blur()
				m.message = ""
				return m, nil
			}
		case "p":
			m.err = nil
			if m.state == stateFeed || m.state == stateProfile || m.err != nil {
				m = m.resetFeed()
				m.state = stateProfile
				m.isLoading = true
				m.message = ""
				return m, m.fetchMyProfileCmd()
			}
		case "r":
			m.err = nil
			if m.state == stateFeed || m.err != nil {
				m = m.resetFeed()
				m.isLoading = true
				if m.feedTitle == "HOT FEED" {
					return m, m.fetchFeedCmd()
				}
				return m, m.fetchPersonalizedFeedCmd()
			}
		case "f":
			m.err = nil
			if m.state == stateFeed || m.err != nil {
				m = m.resetFeed()
				m.feedTitle = "PERSONALIZED FEED"
				m.isLoading = true
				return m, m.fetchPersonalizedFeedCmd()
			}
		case "h":
			m.err = nil
			if m.state == stateFeed || m.err != nil {
				m = m.resetFeed()
				m.feedTitle = "HOT FEED"
				m.isLoading = true
				m.message = ""
				return m, m.fetchFeedCmd()
			}

		case "enter":
			if (m.state == stateFeed || m.state == stateProfile) && len(m.posts) > 0 && m.selectedIndex >= 0 && m.selectedIndex < len(m.posts) {
				m.selectedPost = &m.posts[m.selectedIndex]
				m.state = statePostDetail
				m.isLoadingComments = true
				m.commentIndex = 0
				m.comments = nil // Clear cache
				m.message = ""
				m.viewport.GotoTop()
				
				
				// Force immediate content update to clear stale view buffer
				if m.viewport.Width > 0 {
					content, _ := m.renderDetailContent()
					m.viewport.SetContent(content)
				}
				
				m.ready = false // Force re-init of detail viewport if needed
				return m, m.fetchCommentsCmd(m.selectedPost.ID)
			}
		case "u":
			if (m.state == stateFeed || m.state == stateProfile) && len(m.posts) > 0 && m.selectedIndex >= 0 && m.selectedIndex < len(m.posts) {
				return m, m.upvoteCmd(m.posts[m.selectedIndex].ID)
			}
		case "n":
			m.err = nil
			m.state = stateCreatePost
			m.textInput.Focus()
			m.textInput.SetValue("")
			m.createStep = 0
			m.textInput.Placeholder = "Title"
			return m, nil
		}

	case spinner.TickMsg:
		var spinCmd tea.Cmd
		m.spinner, spinCmd = m.spinner.Update(msg)
		cmd = tea.Batch(cmd, spinCmd)

	case configLoadedMsg:
		m.config = msg.config
		m.client = msg.client
		m.state = stateFeed
		m.isLoading = true
		return m, m.fetchFeedCmd()

	case sessionState:
		m.state = msg
		switch m.state {
		case stateFeed:
			m.isLoading = true
			return m, m.fetchFeedCmd()
		case stateCreatePost:
			m.textInput.Focus()
			m.textInput.SetValue("")
			m.createStep = 0
			m.textInput.Placeholder = "Title"
		case stateRegister:
			m.textInput.Focus()
			m.textInput.Placeholder = "Agent Name"
			m.textInput.SetValue("")
			m.regStep = stepName
		case stateCreateComment:
			m.textInput.Focus()
			m.textInput.SetValue("")
			m.textInput.Placeholder = "Share your thoughts..."
		}

	case feedMsg:
		m.isLoading = false
		m.isPaginating = false
		if !msg.append {
			m.allPostsLoaded = false
		}

		if msg.err != nil {
			if !msg.append {
				m.err = msg.err
			} else {
				// Don't show the error, instead retry automatically after a short delay
				m.isPaginating = true
				m.paginationErr = nil
				return m, func() tea.Msg {
					time.Sleep(3 * time.Second)
					return m.loadMoreCmd()()
				}
			}
		} else {
			m.paginationErr = nil
			if msg.append {
				if len(msg.posts) > 0 {
					m.posts = append(m.posts, msg.posts...)
				}
				if len(msg.posts) < 20 {
					m.allPostsLoaded = true
				}
			} else {
				m.posts = msg.posts
				m.selectedIndex = 0
				m.feedViewport.GotoTop()
				if len(m.posts) < 20 {
					m.allPostsLoaded = true
				}
			}
			m.offset = len(m.posts)
			m.err = nil
		}
		
		// Update feed viewport content
		if m.feedViewport.Width > 0 {
			content, _ := m.renderFeedContent()
			m.feedViewport.SetContent(content)
		}

	case profileMsg:
		m.isLoading = false
		m.isSubmitting = false
		m.regAgent = msg.agent
		m.posts = msg.posts
		m.err = msg.err
		m.selectedIndex = 0
		m.feedViewport.GotoTop()



	case commentsMsg:
		// Ignore stale messages from previous posts
		if m.selectedPost == nil || msg.postID != "" && msg.postID != m.selectedPost.ID {
			return m, nil
		}

		m.isLoading = false
		m.isSubmitting = false
		m.isLoadingComments = false
		
		if msg.err != nil {
			// If appending fails, we could use paginationErr too, or just toast
			if !msg.append {
				m.err = msg.err
			} else {
				m.message = "Failed to load comments: " + msg.err.Error()
			}
		} else {
			if msg.append {
				m.comments = append(m.comments, msg.comments...)
			} else {
				m.comments = msg.comments
			}
			m.err = nil
		}
		
		// Essential: Update viewport with new content
		if m.viewport.Width > 0 {
			content, _ := m.renderDetailContent()
			m.viewport.SetContent(content)
		}

	case postCreatedMsg:
		m.isLoading = false
		m.isSubmitting = false
		if msg.err != nil {
			m.err = msg.err
		}
		m.state = stateFeed
		m.isLoading = true
		return m, m.fetchFeedCmd()

	case commentCreatedMsg:
		m.isSubmitting = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.state = statePostDetail
		m.textInput.Blur()
		m.textInput.SetValue("")
		m.isLoadingComments = true
		m.comments = nil // Clear cache to reload
		// We re-fetch comments
		return m, m.fetchCommentsCmd(m.selectedPost.ID)

	case messageMsg:
		m.message = string(msg)
		return m, nil

	case upvoteSuccessMsg:
		id := string(msg)
		m.upvotedPosts[id] = true
		for i := range m.posts {
			if m.posts[i].ID == id {
				m.posts[i].Upvotes++
			}
		}
		if m.selectedPost != nil && m.selectedPost.ID == id {
			m.selectedPost.Upvotes++
		}

		// ESSENTIAL: Update viewports to show the [UPVOTED] tag and new count
		if m.feedViewport.Width > 0 {
			content, _ := m.renderFeedContent()
			m.feedViewport.SetContent(content)
		}
		if m.viewport.Width > 0 {
			content, _ := m.renderDetailContent()
			m.viewport.SetContent(content)
		}

		return m, nil

	case errMsg:
		m.isLoading = false
		m.isPaginating = false
		m.isSubmitting = false
		m.err = msg.err
		return m, nil
	}

	// View specific updates
	switch m.state {
	case stateFeed:
		m, cmd = m.updateFeed(msg)
	case statePostDetail:
		m, cmd = m.updatePostDetail(msg)
	case stateCreateComment:
		m, cmd = m.updateCreateComment(msg)
	case stateCreatePost:
		m, cmd = m.updateCreatePost(msg)
	case stateRegister:
		m, cmd = m.updateRegister(msg)
	case stateProfile:
		m, cmd = m.updateProfile(msg)
	}

	return m, cmd
}

type feedMsg struct {
	posts  []api.Post
	err    error
	append bool
}



func (m Model) resetFeed() Model {
	m.offset = 0
	m.posts = []api.Post{}
	m.selectedIndex = 0
	m.feedViewport.GotoTop()
	return m
}

func (m Model) fetchFeedCmd() tea.Cmd {
	return func() tea.Msg {
		if m.client == nil {
			return feedMsg{err: fmt.Errorf("client not initialized")}
		}
		posts, err := m.client.GetFeed("hot", 20, m.offset)
		return feedMsg{posts: posts, err: err, append: m.offset > 0}
	}
}

func (m Model) loadMoreCmd() tea.Cmd {
	return func() tea.Msg {
		if m.client == nil {
			return feedMsg{err: fmt.Errorf("client not initialized")}
		}
		
		var posts []api.Post
		var err error
		
		// Use current post count as offset
		currentOffset := len(m.posts)
		
		if m.feedTitle == "HOT FEED" {
			posts, err = m.client.GetFeed("hot", 20, currentOffset)
		} else if m.feedTitle == "SEARCH RESULTS" {
			// Search doesn't currently support offset in this client easily, return empty to reset spinner
			return func() tea.Msg { return feedMsg{append: true} }
		} else {
			posts, err = m.client.GetPersonalizedFeed("hot", 20, currentOffset)
		}
		
		return feedMsg{posts: posts, err: err, append: true}
	}
}

func (m Model) fetchPersonalizedFeedCmd() tea.Cmd {
	return func() tea.Msg {
		if m.client == nil {
			return feedMsg{err: fmt.Errorf("client not initialized")}
		}
		posts, err := m.client.GetPersonalizedFeed("hot", 20, m.offset)
		return feedMsg{posts: posts, err: err, append: m.offset > 0}
	}
}

type errMsg struct{ err error }

func (m Model) View() string {
	if m.err != nil {
		return lipgloss.JoinVertical(lipgloss.Left,
			TitleStyle.Background(lipgloss.Color("#ff0000")).Render(" ERROR "),
			"\n"+lipgloss.NewStyle().Foreground(lipgloss.Color("#ff5555")).Bold(true).Render(m.err.Error()),
			"\n"+HelpStyle.Render("Press 'r' to retry • 'q' to quit"),
		)
	}

	// Full-screen loading only if we have no content and aren't in a creation state
	if m.isLoading && len(m.posts) == 0 && m.state != stateCreatePost && m.state != stateRegister {
		return fmt.Sprintf("\n\n   %s Loading Moltbook...\n   Please wait, AI swarms are busy...\n\n", m.spinner.View())
	}

	switch m.state {
	case stateLoading:
		return "Loading..."
	case stateFeed:
		return m.feedView()
	case statePostDetail:
		return m.postDetailView()
	case stateCreateComment:
		return m.createCommentView()
	case stateCreatePost:
		return m.createPostView()
	case stateRegister:
		return m.registerView()
	case stateProfile:
		return m.profileView()
	default:
		return "Unknown state"
	}
}
