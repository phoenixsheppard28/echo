package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type ChatMessage struct {
	ID      string `json:"id"`
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Prompt   string        `json:"prompt"`
	Messages []ChatMessage `json:"messages"`
}

type ChatResponse struct {
	Message string            `json:"message"`
	Actions []AssistantAction `json:"actions"`
	Error   string            `json:"error"`
}

type AssistantAction struct {
	Kind    string `json:"kind"`
	Status  string `json:"status"`
	Title   string `json:"title"`
	Detail  string `json:"detail"`
	EventID string `json:"eventId"`
	Link    string `json:"link"`
	Time    string `json:"time"`
}

type WindowModeResponse struct {
	Mode   string `json:"mode"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type Assistant struct {
	calendar *GoogleCalendarClient
	ollama   *OllamaClient
}

func (a *Assistant) Chat(req ChatRequest) ChatResponse {
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return ChatResponse{Message: "Ask me something or tell me what to schedule."}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	plan, raw, err := a.ollama.Plan(ctx, req.Messages, prompt)
	if err != nil {
		return ChatResponse{
			Message: "I could not reach the local llama3 model through Ollama. Make sure Ollama is running, then try again.",
			Error:   err.Error(),
			Actions: []AssistantAction{{
				Kind:   "model",
				Status: "failed",
				Title:  "Ollama unavailable",
				Detail: err.Error(),
				Time:   actionTime(),
			}},
		}
	}

	if plan.Calendar == nil || plan.Calendar.Action == "" || plan.Calendar.Action == "none" {
		if strings.TrimSpace(plan.Response) == "" {
			plan.Response = strings.TrimSpace(raw)
		}
		return ChatResponse{Message: cleanAssistantText(plan.Response)}
	}

	switch plan.Calendar.Action {
	case "list_events":
		return a.handleListEvents(ctx, plan)
	case "create_event":
		return a.handleCreateEvent(ctx, plan)
	default:
		return ChatResponse{Message: fallbackResponse(plan.Response)}
	}
}

func (a *Assistant) handleListEvents(ctx context.Context, plan modelPlan) ChatResponse {
	timeMin, timeMax := defaultListRange()
	if parsed, ok := parseModelTime(plan.Calendar.TimeMin); ok {
		timeMin = parsed
	}
	if parsed, ok := parseModelTime(plan.Calendar.TimeMax); ok {
		timeMax = parsed
	}

	events, err := a.calendar.ListEvents(ctx, timeMin, timeMax, 12)
	action := AssistantAction{
		Kind:   "calendar",
		Title:  "Checked Google Calendar",
		Detail: fmt.Sprintf("%s to %s", formatWhen(timeMin), formatWhen(timeMax)),
		Time:   actionTime(),
	}
	if err != nil {
		action.Status = "failed"
		action.Detail = err.Error()
		return ChatResponse{
			Message: calendarErrorMessage(err),
			Actions: []AssistantAction{
				action,
			},
			Error: err.Error(),
		}
	}

	action.Status = "done"
	return ChatResponse{
		Message: formatCalendarEvents(events, timeMin, timeMax),
		Actions: []AssistantAction{
			action,
		},
	}
}

func (a *Assistant) handleCreateEvent(ctx context.Context, plan modelPlan) ChatResponse {
	title := strings.TrimSpace(plan.Calendar.Title)
	start, startOK := parseModelTime(plan.Calendar.Start)
	end, endOK := parseModelTime(plan.Calendar.End)

	if title == "" || !startOK {
		return ChatResponse{
			Message: "I need a title and a start time before I can put that on your calendar.",
			Actions: []AssistantAction{{
				Kind:   "calendar",
				Status: "blocked",
				Title:  "Calendar event needs details",
				Detail: "Missing title or start time",
				Time:   actionTime(),
			}},
		}
	}
	if !endOK {
		end = start.Add(time.Hour)
	}

	event, err := a.calendar.CreateEvent(ctx, CalendarEventInput{
		Title:       title,
		Description: plan.Calendar.Description,
		Location:    plan.Calendar.Location,
		Start:       start,
		End:         end,
		Attendees:   plan.Calendar.Attendees,
	})

	action := AssistantAction{
		Kind:   "calendar",
		Title:  "Created Google Calendar event",
		Detail: fmt.Sprintf("%s, %s to %s", title, formatWhen(start), formatClock(end)),
		Time:   actionTime(),
	}
	if err != nil {
		action.Status = "failed"
		action.Detail = err.Error()
		return ChatResponse{
			Message: calendarErrorMessage(err),
			Actions: []AssistantAction{
				action,
			},
			Error: err.Error(),
		}
	}

	action.Status = "done"
	action.EventID = event.ID
	action.Link = event.Link
	return ChatResponse{
		Message: fmt.Sprintf("Done. I added \"%s\" to Google Calendar for %s.", title, formatWhen(start)),
		Actions: []AssistantAction{
			action,
		},
	}
}

type OllamaClient struct {
	model string
	url   string
	http  *http.Client
}

func NewOllamaClient(model string) *OllamaClient {
	return &OllamaClient{
		model: model,
		url:   "http://localhost:11434/api/chat",
		http:  &http.Client{Timeout: 85 * time.Second},
	}
}

func (c *OllamaClient) Plan(ctx context.Context, history []ChatMessage, prompt string) (modelPlan, string, error) {
	messages := []ollamaMessage{{
		Role:    "system",
		Content: assistantSystemPrompt(),
	}}

	for _, msg := range lastMessages(history, 10) {
		role := msg.Role
		if role != "user" && role != "assistant" {
			continue
		}
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		messages = append(messages, ollamaMessage{Role: role, Content: content})
	}
	if len(messages) == 1 || strings.TrimSpace(history[len(history)-1].Content) != prompt {
		messages = append(messages, ollamaMessage{Role: "user", Content: prompt})
	}

	payload := ollamaChatRequest{
		Model:    c.model,
		Messages: messages,
		Stream:   false,
		Format:   "json",
		Options: map[string]any{
			"temperature": 0.1,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return modelPlan{}, "", err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return modelPlan{}, "", err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := c.http.Do(request)
	if err != nil {
		return modelPlan{}, "", err
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return modelPlan{}, "", err
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return modelPlan{}, "", fmt.Errorf("ollama returned %s: %s", response.Status, strings.TrimSpace(string(responseBody)))
	}

	var decoded ollamaChatResponse
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return modelPlan{}, "", err
	}

	raw := strings.TrimSpace(decoded.Message.Content)
	plan, err := parseModelPlan(raw)
	return plan, raw, err
}

type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Format   string          `json:"format"`
	Options  map[string]any  `json:"options,omitempty"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaChatResponse struct {
	Message ollamaMessage `json:"message"`
}

type modelPlan struct {
	Response string        `json:"response"`
	Calendar *calendarPlan `json:"calendar"`
}

type calendarPlan struct {
	Action      string   `json:"action"`
	Title       string   `json:"title"`
	Start       string   `json:"start"`
	End         string   `json:"end"`
	TimeMin     string   `json:"time_min"`
	TimeMax     string   `json:"time_max"`
	Description string   `json:"description"`
	Location    string   `json:"location"`
	Attendees   []string `json:"attendees"`
}

func assistantSystemPrompt() string {
	now := time.Now()
	return fmt.Sprintf(`You are Echo, a concise harnessed desktop assistant.
You run locally through Ollama. The current local time is %s. The local timezone is %s.

You may use exactly one tool family: Google Calendar.

Return only one JSON object with this shape:
{
  "response": "short message to show the user",
  "calendar": {
    "action": "none | list_events | create_event",
    "title": "",
    "start": "",
    "end": "",
    "time_min": "",
    "time_max": "",
    "description": "",
    "location": "",
    "attendees": []
  }
}

Rules:
- Use action "list_events" when the user asks what is scheduled, whether they are free, or wants upcoming calendar items.
- Use action "create_event" only when the user clearly asks to add, book, schedule, or create a calendar event.
- For create_event, include a specific title and RFC3339 start/end datetime with timezone offset. If duration is missing, omit end and the app will use one hour.
- If a create_event request is missing the date or time, use action "none" and ask one concise clarifying question in response.
- For list_events, include RFC3339 time_min and time_max. If the user is vague, choose now through seven days from now.
- Do not claim an event was created or read until the app executes the calendar action.
- If the user asks for anything outside Calendar, answer normally with action "none".`, now.Format(time.RFC3339), now.Location().String())
}

func parseModelPlan(raw string) (modelPlan, error) {
	var plan modelPlan
	cleaned := extractJSONObject(raw)
	if cleaned == "" {
		return plan, fmt.Errorf("model did not return JSON")
	}
	if err := json.Unmarshal([]byte(cleaned), &plan); err != nil {
		return plan, err
	}
	if plan.Calendar != nil {
		plan.Calendar.Action = strings.TrimSpace(plan.Calendar.Action)
	}
	return plan, nil
}

func extractJSONObject(raw string) string {
	text := strings.TrimSpace(raw)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)

	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end < start {
		return ""
	}
	return text[start : end+1]
}

func lastMessages(messages []ChatMessage, count int) []ChatMessage {
	if len(messages) <= count {
		return messages
	}
	return messages[len(messages)-count:]
}

func defaultListRange() (time.Time, time.Time) {
	now := time.Now()
	return now, now.AddDate(0, 0, 7)
}

func parseModelTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, true
	}
	if parsed, err := time.Parse("2006-01-02 15:04", value); err == nil {
		return parsed, true
	}
	return time.Time{}, false
}

func formatCalendarEvents(events []CalendarEventSummary, timeMin, timeMax time.Time) string {
	if len(events) == 0 {
		return fmt.Sprintf("You have no Google Calendar events from %s to %s.", formatWhen(timeMin), formatWhen(timeMax))
	}

	lines := []string{"Here is what I found on Google Calendar:"}
	for _, event := range events {
		lines = append(lines, fmt.Sprintf("- %s, %s", event.Title, event.When))
	}
	return strings.Join(lines, "\n")
}

func calendarErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	if IsCalendarAuthError(err) {
		return "Google Calendar is not connected yet. Use Connect Calendar, then try again."
	}
	return "I could not complete the Google Calendar action: " + err.Error()
}

func fallbackResponse(response string) string {
	if strings.TrimSpace(response) == "" {
		return "I can help with Google Calendar right now. Tell me what to schedule or ask what is coming up."
	}
	return cleanAssistantText(response)
}

func cleanAssistantText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "Done."
	}
	return value
}

func actionTime() string {
	return time.Now().Format("3:04 PM")
}

func formatWhen(value time.Time) string {
	return value.Local().Format("Mon Jan 2, 3:04 PM")
}

func formatClock(value time.Time) string {
	return value.Local().Format("3:04 PM")
}
