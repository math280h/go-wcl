// Command introspect fetches the Warcraft Logs GraphQL schema via introspection
// and writes it to schema/schema.graphql for genqlient to consume.
//
// Run from the tools directory:
//
//	go run ./introspect
//
// Credentials are read from the environment (WCL_CLIENT_ID, WCL_CLIENT_SECRET),
// falling back to a .env file in the repository root.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/suessflorian/gqlfetch"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

const (
	tokenURL = "https://www.warcraftlogs.com/oauth/token"
	endpoint = "https://www.warcraftlogs.com/api/v2/client"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "introspect:", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()
	root := repoRoot()

	if envPath := filepath.Join(root, ".env"); fileExists(envPath) {
		if err := godotenv.Load(envPath); err != nil {
			return fmt.Errorf("load %s: %w", envPath, err)
		}
	}

	id, secret := os.Getenv("WCL_CLIENT_ID"), os.Getenv("WCL_CLIENT_SECRET")
	if id == "" || secret == "" {
		return fmt.Errorf("WCL_CLIENT_ID and WCL_CLIENT_SECRET must be set (see .env.example)")
	}

	cc := clientcredentials.Config{
		ClientID:     id,
		ClientSecret: secret,
		TokenURL:     tokenURL,
		AuthStyle:    oauth2.AuthStyleInHeader,
	}
	tok, err := cc.Token(ctx)
	if err != nil {
		return fmt.Errorf("fetch token: %w", err)
	}

	headers := http.Header{}
	headers.Set("Authorization", tok.Type()+" "+tok.AccessToken)
	headers.Set("Accept", "application/json")

	schema, err := gqlfetch.BuildClientSchemaWithHeaders(ctx, endpoint, headers, true)
	if err != nil {
		return fmt.Errorf("introspect schema: %w", err)
	}

	out := filepath.Join(root, "schema", "schema.graphql")
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(out, []byte(schema), 0o644); err != nil {
		return err
	}

	fmt.Printf("wrote %s (%d bytes)\n", out, len(schema))
	return nil
}

func repoRoot() string {
	if fileExists("genqlient.yaml") {
		return "."
	}
	return ".."
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
