package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	echooauth "echo/oauth"

	"github.com/pkg/browser"
	"golang.org/x/oauth2"
	googleoauth "golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

const (
	googleCalendarAccountID = "google-calendar-primary"
	googleCalendarProvider  = "google"
	googleRedirectURL       = "http://localhost:8080/callback"
)

var errCalendarAuthRequired = errors.New("google calendar is not connected")

type CalendarStatus struct {
	Connected bool   `json:"connected"`
	Message   string `json:"message"`
	Account   string `json:"account"`
	Error     string `json:"error"`
}

type CalendarEventSummary struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	When  string `json:"when"`
	Link  string `json:"link"`
}

type CalendarEventInput struct {
	Title       string
	Description string
	Location    string
	Start       time.Time
	End         time.Time
	Attendees   []string
}

type GoogleCalendarClient struct {
	mu     sync.Mutex
	config *oauth2.Config
	token  *oauth2.Token
	store  echooauth.TokenStore
}

func NewGoogleCalendarClient() *GoogleCalendarClient {
	return &GoogleCalendarClient{
		store: echooauth.TokenStore{Service: echooauth.SERVICE},
	}
}

func (c *GoogleCalendarClient) Status() CalendarStatus {
	if _, err := c.oauthConfig(); err != nil {
		return CalendarStatus{
			Connected: false,
			Message:   "Google Calendar credentials were not found.",
			Error:     err.Error(),
		}
	}
	if _, err := c.loadToken(); err != nil {
		return CalendarStatus{
			Connected: false,
			Message:   "Google Calendar is ready to connect.",
			Error:     err.Error(),
		}
	}
	return CalendarStatus{
		Connected: true,
		Message:   "Google Calendar connected.",
		Account:   googleCalendarAccountID,
	}
}

func (c *GoogleCalendarClient) Connect() CalendarStatus {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	token, err := c.runOAuth(ctx)
	if err != nil {
		return CalendarStatus{
			Connected: false,
			Message:   "Google Calendar connection failed.",
			Error:     err.Error(),
		}
	}
	if err := c.saveToken(token); err != nil {
		return CalendarStatus{
			Connected: false,
			Message:   "Google Calendar authorized, but the token could not be saved.",
			Error:     err.Error(),
		}
	}

	c.mu.Lock()
	c.token = token
	c.mu.Unlock()

	return CalendarStatus{
		Connected: true,
		Message:   "Google Calendar connected.",
		Account:   googleCalendarAccountID,
	}
}

func (c *GoogleCalendarClient) ListEvents(ctx context.Context, timeMin, timeMax time.Time, maxResults int64) ([]CalendarEventSummary, error) {
	service, err := c.service(ctx)
	if err != nil {
		return nil, err
	}
	if maxResults <= 0 {
		maxResults = 10
	}

	events, err := service.Events.List("primary").
		ShowDeleted(false).
		SingleEvents(true).
		TimeMin(timeMin.Format(time.RFC3339)).
		TimeMax(timeMax.Format(time.RFC3339)).
		MaxResults(maxResults).
		OrderBy("startTime").
		Do()
	if err != nil {
		return nil, err
	}

	summaries := make([]CalendarEventSummary, 0, len(events.Items))
	for _, item := range events.Items {
		summaries = append(summaries, CalendarEventSummary{
			ID:    item.Id,
			Title: eventTitle(item.Summary),
			When:  eventWhen(item.Start, item.End),
			Link:  item.HtmlLink,
		})
	}
	return summaries, nil
}

func (c *GoogleCalendarClient) CreateEvent(ctx context.Context, input CalendarEventInput) (CalendarEventSummary, error) {
	service, err := c.service(ctx)
	if err != nil {
		return CalendarEventSummary{}, err
	}

	attendees := make([]*calendar.EventAttendee, 0, len(input.Attendees))
	for _, attendee := range input.Attendees {
		email := strings.TrimSpace(attendee)
		if email != "" {
			attendees = append(attendees, &calendar.EventAttendee{Email: email})
		}
	}

	event := &calendar.Event{
		Summary:     input.Title,
		Description: input.Description,
		Location:    input.Location,
		Start: &calendar.EventDateTime{
			DateTime: input.Start.Format(time.RFC3339),
			TimeZone: input.Start.Location().String(),
		},
		End: &calendar.EventDateTime{
			DateTime: input.End.Format(time.RFC3339),
			TimeZone: input.End.Location().String(),
		},
		Attendees: attendees,
	}

	created, err := service.Events.Insert("primary", event).Do()
	if err != nil {
		return CalendarEventSummary{}, err
	}

	return CalendarEventSummary{
		ID:    created.Id,
		Title: eventTitle(created.Summary),
		When:  eventWhen(created.Start, created.End),
		Link:  created.HtmlLink,
	}, nil
}

func (c *GoogleCalendarClient) service(ctx context.Context) (*calendar.Service, error) {
	cfg, err := c.oauthConfig()
	if err != nil {
		return nil, err
	}

	token, err := c.loadToken()
	if err != nil {
		return nil, errCalendarAuthRequired
	}
	source := cfg.TokenSource(ctx, token)

	refreshed, err := source.Token()
	if err != nil {
		return nil, err
	}
	if refreshed.AccessToken != token.AccessToken {
		_ = c.saveToken(refreshed)
		c.mu.Lock()
		c.token = refreshed
		c.mu.Unlock()
	}

	return calendar.NewService(ctx, option.WithTokenSource(source))
}

func (c *GoogleCalendarClient) oauthConfig() (*oauth2.Config, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.config != nil {
		return c.config, nil
	}

	credentialsPath, err := findCredentialsPath()
	if err != nil {
		return nil, err
	}
	body, err := os.ReadFile(credentialsPath)
	if err != nil {
		return nil, err
	}

	cfg, err := googleoauth.ConfigFromJSON(body, calendar.CalendarEventsScope)
	if err != nil {
		return nil, err
	}
	cfg.RedirectURL = googleRedirectURL
	cfg.Endpoint.AuthStyle = oauth2.AuthStyleInParams
	c.config = cfg
	return c.config, nil
}

func (c *GoogleCalendarClient) loadToken() (*oauth2.Token, error) {
	c.mu.Lock()
	if c.token != nil && c.token.Valid() {
		token := c.token
		c.mu.Unlock()
		return token, nil
	}
	c.mu.Unlock()

	if token, err := c.loadKeyringToken(); err == nil {
		c.mu.Lock()
		c.token = token
		c.mu.Unlock()
		return token, nil
	}

	token, err := c.loadFileToken()
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.token = token
	c.mu.Unlock()
	return token, nil
}

func (c *GoogleCalendarClient) loadKeyringToken() (*oauth2.Token, error) {
	data, err := c.store.Load(googleCalendarProvider, googleCalendarAccountID)
	if err != nil {
		return nil, err
	}
	var token oauth2.Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, err
	}
	return &token, nil
}

func (c *GoogleCalendarClient) loadFileToken() (*oauth2.Token, error) {
	path, err := tokenFilePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var token oauth2.Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, err
	}
	return &token, nil
}

func (c *GoogleCalendarClient) saveToken(token *oauth2.Token) error {
	data, err := json.Marshal(token)
	if err != nil {
		return err
	}

	keyringErr := c.store.Save(googleCalendarProvider, googleCalendarAccountID, data)
	path, filePathErr := tokenFilePath()
	if filePathErr != nil {
		if keyringErr != nil {
			return fmt.Errorf("keyring: %v; config path: %v", keyringErr, filePathErr)
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		if keyringErr != nil {
			return fmt.Errorf("keyring: %v; config dir: %v", keyringErr, err)
		}
		return nil
	}
	if err := os.WriteFile(path, data, 0600); err != nil && keyringErr != nil {
		return fmt.Errorf("keyring: %v; token file: %v", keyringErr, err)
	}
	return nil
}

func (c *GoogleCalendarClient) runOAuth(ctx context.Context) (*oauth2.Token, error) {
	cfg, err := c.oauthConfig()
	if err != nil {
		return nil, err
	}

	state, err := randomState()
	if err != nil {
		return nil, err
	}
	verifier := oauth2.GenerateVerifier()
	authURL := cfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.S256ChallengeOption(verifier))

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			http.Error(w, "State did not match. You can close this tab.", http.StatusBadRequest)
			errCh <- errors.New("oauth state did not match")
			return
		}
		if oauthErr := r.URL.Query().Get("error"); oauthErr != "" {
			http.Error(w, "Google Calendar authorization failed. You can close this tab.", http.StatusBadRequest)
			errCh <- fmt.Errorf("oauth error: %s", oauthErr)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "Missing OAuth code. You can close this tab.", http.StatusBadRequest)
			errCh <- errors.New("missing oauth code")
			return
		}
		fmt.Fprintln(w, "Echo is connected to Google Calendar. You can close this tab.")
		codeCh <- code
	})

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	if err := browser.OpenURL(authURL); err != nil {
		return nil, err
	}

	select {
	case code := <-codeCh:
		return cfg.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	case err := <-errCh:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func randomState() (string, error) {
	buffer := make([]byte, 64)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	sum := sha256.Sum256(buffer)
	return hex.EncodeToString(sum[:]), nil
}

func findCredentialsPath() (string, error) {
	candidates := []string{
		filepath.Join("oauth", "credentials.json"),
		"credentials.json",
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", errors.New("oauth/credentials.json was not found")
}

func tokenFilePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "echo", "google-calendar-token.json"), nil
}

func eventTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "Untitled event"
	}
	return title
}

func eventWhen(start, end *calendar.EventDateTime) string {
	startText := eventTime(start)
	endText := eventTime(end)
	if startText == "" {
		return "Time not set"
	}
	if endText == "" || endText == startText {
		return startText
	}
	return startText + " to " + endText
}

func eventTime(value *calendar.EventDateTime) string {
	if value == nil {
		return ""
	}
	if value.DateTime != "" {
		if parsed, err := time.Parse(time.RFC3339, value.DateTime); err == nil {
			return parsed.Local().Format("Mon Jan 2, 3:04 PM")
		}
		return value.DateTime
	}
	if value.Date != "" {
		if parsed, err := time.Parse("2006-01-02", value.Date); err == nil {
			return parsed.Format("Mon Jan 2")
		}
		return value.Date
	}
	return ""
}

func IsCalendarAuthError(err error) bool {
	return errors.Is(err, errCalendarAuthRequired)
}
