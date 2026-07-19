// Command userauth runs the OAuth 2.0 authorization-code flow with PKCE and
// calls an endpoint that only works with user authentication.
//
//	export WCL_CLIENT_ID=...
//	export WCL_CLIENT_SECRET=...   # omit for a public client (PKCE only)
//	go run ./examples/userauth
//
// The redirect URI must be registered on the client management page at
// https://www.warcraftlogs.com/api/clients/.
package main

import (
	"context"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"time"

	"golang.org/x/oauth2"

	warcraftlogs "github.com/math280h/go-wcl"
)

const authTimeout = 10 * time.Minute

func main() {
	redirect := flag.String("redirect", "http://localhost:8080/callback", "redirect URI registered on the client")
	flag.Parse()

	if err := run(*redirect); err != nil {
		log.Fatal(err)
	}
}

func run(redirect string) error {
	id, secret := os.Getenv("WCL_CLIENT_ID"), os.Getenv("WCL_CLIENT_SECRET")
	if id == "" {
		return errors.New("set WCL_CLIENT_ID")
	}
	redirectURL, err := url.Parse(redirect)
	if err != nil {
		return fmt.Errorf("parse redirect: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, authTimeout)
	defer cancel()

	cfg := warcraftlogs.OAuthConfig(id, secret, redirect, "view-user-profile")
	state := rand.Text()
	verifier := oauth2.GenerateVerifier()

	callbacks := make(chan callback, 1)
	srv, err := serve(redirectURL, callbacks)
	if err != nil {
		return err
	}
	defer srv.Shutdown(context.Background()) //nolint:errcheck

	fmt.Println("Open this URL to authorize:")
	fmt.Println()
	fmt.Println("  " + cfg.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier)))
	fmt.Println()
	fmt.Println("Waiting for the callback...")

	var cb callback
	select {
	case cb = <-callbacks:
	case <-ctx.Done():
		return ctx.Err()
	}
	if cb.err != nil {
		return cb.err
	}
	if cb.state != state {
		return errors.New("state mismatch, discarding the response")
	}

	token, err := cfg.Exchange(ctx, cb.code, oauth2.VerifierOption(verifier))
	if err != nil {
		return fmt.Errorf("exchange code: %w", err)
	}

	client, err := warcraftlogs.New(ctx,
		warcraftlogs.WithTokenSource(cfg.TokenSource(context.WithoutCancel(ctx), token)),
		warcraftlogs.WithEndpoint(warcraftlogs.UserEndpoint))
	if err != nil {
		return err
	}

	user, err := client.CurrentUser(ctx)
	if err != nil {
		return fmt.Errorf("current user: %w", err)
	}

	fmt.Printf("\nauthenticated as %s (id %d)\n", user.Name, user.Id)
	if user.BattleTag != "" {
		fmt.Printf("battle tag: %s\n", user.BattleTag)
	}
	fmt.Printf("token expires: %s\n", token.Expiry.Format(time.RFC3339))
	return nil
}

type callback struct {
	code  string
	state string
	err   error
}

// serve handles a single OAuth redirect on the host and path of u.
func serve(u *url.URL, out chan<- callback) (*http.Server, error) {
	ln, err := net.Listen("tcp", u.Host)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", u.Host, err)
	}

	path := u.Path
	if path == "" {
		path = "/"
	}

	mux := http.NewServeMux()
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("code") == "" && q.Get("error") == "" {
			http.NotFound(w, r)
			return
		}

		cb := callback{code: q.Get("code"), state: q.Get("state")}
		if e := q.Get("error"); e != "" {
			cb.err = fmt.Errorf("authorization refused: %s %s", e, q.Get("error_description"))
		}

		if cb.err != nil {
			http.Error(w, cb.err.Error(), http.StatusBadRequest)
		} else {
			fmt.Fprintln(w, "Authorized. You can close this tab.")
		}
		select {
		case out <- cb:
		default:
		}
	})

	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go srv.Serve(ln) //nolint:errcheck
	return srv, nil
}
