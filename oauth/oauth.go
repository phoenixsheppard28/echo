package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/pkg/browser"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

func generateState() (string, error) {
	b := make([]byte, 1024)

	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	hash := sha256.Sum256(b)
	return hex.EncodeToString(hash[:]), nil
}

/*
http://localhost/callback?
state=860f26a7fca269cb9073da0ebce573fea4f56cc5620ec9019e9498b54c54d8e1&
iss=https://accounts.google.com&
code=4/0AeoWuM_deo4NSNKT5jh8J_WuKDtvZhuCWAAwpyhoxmvo90613seXt9drAYmyO678CLjk_g&
scope=https://www.googleapis.com/auth/calendar.events.owned
*/
func waitForCallback(termChan <-chan struct{}, codeChan chan<- string, errChan chan<- error, state string) {

	mux := http.NewServeMux()
	s := http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {

		urlState := r.URL.Query().Get("state")

		if state != urlState {
			errChan <- errors.New("State does not match")
		}

		err := r.URL.Query().Get("error")
		if err != "" {
			if err == "access_denied" {
				fmt.Fprintln(w, "You have denied access, if you wish to regrant please start the flow again from the app")
			}
			fmt.Fprintf(w, "An error has occured: %v", err)
			errChan <- errors.New("error in auth flow")
			return
		}

		code := r.URL.Query().Get("code")

		fmt.Fprintln(w, "Authorization complete. You can close this tab.")

		codeChan <- code

		go func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = s.Shutdown(shutdownCtx)
		}()
	})

	go func() {
		<-termChan
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.Shutdown(shutdownCtx)
	}()

	if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		errChan <- err
	}
}

/*
	Step 1, generate state and code_verifier
	Step 2: generate the authUrl from state, code_verifier and credentials?
	Step 3: serve a quick localhost instance of /callback to handle the request
	Step 4 open a web browser with the AuthUrl
		... user vierifies
	Step 5:

*/

func step1() (string, string) {
	code_verifier := oauth2.GenerateVerifier()
	state, err := generateState()
	if err != nil {
		fmt.Printf("unable to create state token: %v", err)
	}
	return code_verifier, state

}
func step2(config *oauth2.Config, code_verifier string, state string) string {
	// * generate config from credentials (maybe remove secret and see),
	authUrl := config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.S256ChallengeOption(code_verifier))
	return authUrl
}

func step3(authUrl string) {

}

func googleCalenderOath(cfg *oauth2.Config) (*oauth2.Token, error) {

	verifier, state := step1()

	authUrl := step2(cfg, verifier, state)

	// make a web server to handle the callback
	termChan := make(chan struct{})
	codeChan := make(chan string)
	errChan := make(chan error)

	go waitForCallback(termChan, codeChan, errChan, state)
	defer func() {
		termChan <- struct{}{}
	}()

	fmt.Println("if a browser window does not open, paste this URL into your web browser:")
	fmt.Println(authUrl)
	browser.OpenURL(authUrl)

	var code string
	select {
	case code = <-codeChan:
		fmt.Println(code)

	case err := <-errChan:
		fmt.Printf("Error with server: %v", err)
		return nil, err

	case <-time.After(2 * time.Minute):
		fmt.Printf("timeout, please try again")
		return nil, errors.New("Timeout")
	}

	token, err := cfg.Exchange(context.Background(), code, oauth2.AccessTypeOffline, oauth2.VerifierOption(verifier))
	if err != nil {
		fmt.Println("error exchaning token: %v", err)
	}
	return token, err
}
func main() {

	b, err := os.ReadFile("credentials.json") // maybe replace with other config inline
	if err != nil {
		fmt.Printf("Unable to parse client secret file to config: %v", err)
	}

	config, err := google.ConfigFromJSON(b, calendar.CalendarEventsOwnedScope)
	if err != nil {
		fmt.Printf("Unable to create config: %v", err)
	}
	config.Endpoint.AuthStyle = oauth2.AuthStyleInParams

	token, err := googleCalenderOath(config)
	if err != nil {
		fmt.Printf("Error getting token: %v", err)
	}
	ts := config.TokenSource(context.Background(), token)

	srv, err := calendar.NewService(context.Background(), option.WithTokenSource(ts))
	if err != nil {
		log.Fatalf("Unable to retrieve Calendar client: %v", err)
	}

	t := time.Now().Format(time.RFC3339)
	events, err := srv.Events.List("primary").ShowDeleted(false).
		SingleEvents(true).TimeMin(t).MaxResults(10).OrderBy("startTime").Do()
	if err != nil {
		log.Fatalf("Unable to retrieve next ten of the user's events: %v", err)
	}
	fmt.Println("Upcoming events:")
	if len(events.Items) == 0 {
		fmt.Println("No upcoming events found.")
	} else {
		for _, item := range events.Items {
			date := item.Start.DateTime
			if date == "" {
				date = item.Start.Date
			}
			fmt.Printf("%v (%v)\n", item.Summary, date)
		}
	}

}

// func step5(){

// 	token, err := config.Exchange(
// 		ctx,
// 		code,
// 		oauth2.VerifierOption(verifier),
// 	)
// }

// // Saves a token to a file path.
// func saveToken(path string, token *oauth2.Token) {
// 	fmt.Printf("Saving credential file to: %s\n", path)
// 	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
// 	if err != nil {
// 		log.Fatalf("Unable to cache oauth token: %v", err)
// 	}
// 	defer f.Close()
// 	json.NewEncoder(f).Encode(token)
// }
