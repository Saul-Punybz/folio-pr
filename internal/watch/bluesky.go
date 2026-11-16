package watch

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/Saul-Punybz/folio/internal/models"
)

// Bluesky: búsqueda pública de posts, sin cuenta ni llave. public.api.bsky.app
// responde 403 a searchPosts; api.bsky.app responde 200 (comprobado nov 2026).
const bskyAPI = "https://api.bsky.app/xrpc/"

type bskyPost struct {
	URI    string `json:"uri"`
	Author struct {
		Handle      string `json:"handle"`
		DisplayName string `json:"displayName"`
	} `json:"author"`
	Record struct {
		Text      string   `json:"text"`
		CreatedAt string   `json:"createdAt"`
		Tags      []string `json:"tags"`
		Facets    []struct {
			Features []struct {
				Type string `json:"$type"`
				Tag  string `json:"tag"`
			} `json:"features"`
		} `json:"facets"`
		Embed struct {
			External struct {
				URI         string `json:"uri"`
				Title       string `json:"title"`
				Description string `json:"description"`
			} `json:"external"`
		} `json:"embed"`
	} `json:"record"`
	IndexedAt string `json:"indexedAt"`
}

// socialPost es un post ya normalizado de una red (Bluesky, Mastodon).
type socialPost struct {
	URL       string
	Author    string
	Text      string // texto para mostrar
	MatchText string // texto + enlace incrustado, para la coincidencia
	Tags      []string
	Published time.Time
}

// parseBlueskyPosts lee la respuesta de searchPosts o getAuthorFeed.
func parseBlueskyPosts(body []byte) ([]socialPost, error) {
	var raw struct {
		Posts []bskyPost `json:"posts"`
		Feed  []struct {
			Post bskyPost `json:"post"`
		} `json:"feed"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("bluesky: json: %w", err)
	}
	posts := raw.Posts
	for _, f := range raw.Feed {
		posts = append(posts, f.Post)
	}
	out := make([]socialPost, 0, len(posts))
	for _, p := range posts {
		web := bskyWebURL(p.URI, p.Author.Handle)
		if web == "" {
			continue
		}
		tags := append([]string{}, p.Record.Tags...)
		for _, f := range p.Record.Facets {
			for _, ft := range f.Features {
				if strings.HasSuffix(ft.Type, "#tag") && ft.Tag != "" {
					tags = append(tags, ft.Tag)
				}
			}
		}
		ext := p.Record.Embed.External
		text := strings.TrimSpace(p.Record.Text)
		if text == "" {
			text = strings.TrimSpace(ext.Title)
		}
		author := p.Author.DisplayName
		if author == "" {
			author = p.Author.Handle
		}
		t, _ := time.Parse(time.RFC3339, p.Record.CreatedAt)
		out = append(out, socialPost{
			URL:       web,
			Author:    author,
			Text:      text,
			MatchText: strings.Join([]string{p.Record.Text, ext.Title, ext.Description, ext.URI}, " "),
			Tags:      tags,
			Published: t,
		})
	}
	return out, nil
}

// bskyWebURL: at://did/app.bsky.feed.post/rkey → https://bsky.app/profile/handle/post/rkey
func bskyWebURL(uri, handle string) string {
	parts := strings.Split(strings.TrimPrefix(uri, "at://"), "/")
	if len(parts) != 3 || parts[1] != "app.bsky.feed.post" || parts[2] == "" {
		return ""
	}
	who := handle
	if who == "" || who == "handle.invalid" {
		who = parts[0]
	}
	return "https://bsky.app/profile/" + who + "/post/" + parts[2]
}

// postTitle arma un título corto: «Autor: texto…».
func postTitle(p socialPost) string {
	t := strings.Join(strings.Fields(p.Text), " ")
	if t == "" {
		t = "(sin texto)"
	}
	return truncateStr(p.Author+": "+t, 160)
}

// acceptPost decide si un post de una red global entra como mención, y con
// qué hashtag. Reglas: nunca con exclusiones; si viene de la propia cuenta de
// la marca, entra; si trae un hashtag distintivo de la marca, entra; si no,
// tiene que nombrarla (alias corto o de varias palabras: con contexto).
func acceptPost(p socialPost, org models.WatchlistOrg, m Matcher, own bool) (match, bool) {
	text := p.MatchText + " " + strings.Join(prefixTags(p.Tags), " ")
	if Excluded(text, org) {
		return match{}, false
	}
	if own {
		return match{Tag: findHashtag(text, org)}, true
	}
	m.NeedPR = true
	return matchItem(text, org, m)
}

func prefixTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		out = append(out, "#"+normTag(t))
	}
	return out
}

// blueskyQueries: alias entre comillas y hashtags, como mucho 6.
func blueskyQueries(org models.WatchlistOrg) []string {
	var qs []string
	seen := map[string]bool{}
	add := func(q string) {
		if !seen[strings.ToLower(q)] && len(qs) < 8 {
			seen[strings.ToLower(q)] = true
			qs = append(qs, q)
		}
	}
	for _, a := range matchTerms(org) {
		if weakTerm(a) {
			continue // «PSI» solo trae ruido; lo cubren los otros alias
		}
		if strings.ContainsAny(a, "._@") {
			continue // dominios y handles: los cubren los hashtags
		}
		add(`"` + a + `"`)
		if len(qs) >= 3 {
			break
		}
	}
	for _, h := range org.Hashtags {
		if h = normTag(h); h != "" {
			add("#" + h)
		}
	}
	for _, p := range org.People {
		add(`"` + p.Name + `"`)
	}
	return qs
}

// ScanBluesky busca la marca en Bluesky (texto y hashtags) y lee el feed de
// sus cuentas en Bluesky si tiene.
func ScanBluesky(ctx context.Context, org models.WatchlistOrg, m Matcher, deps Deps) int {
	saved := 0
	run := func(endpoint string, own bool) {
		if saved >= maxResultsPerAgent || ctx.Err() != nil {
			return
		}
		body, code, err := fetch(ctx, endpoint, "application/json")
		if err != nil {
			slog.Warn("watchlist/bluesky: fetch", "org", org.Name, "code", code, "err", err)
			return
		}
		posts, err := parseBlueskyPosts(body)
		if err != nil {
			slog.Warn("watchlist/bluesky: parse", "org", org.Name, "err", err)
			return
		}
		for _, p := range posts {
			if saved >= maxResultsPerAgent {
				return
			}
			mt, ok := acceptPost(p, org, m, own)
			if !ok {
				continue
			}
			if saveHit(ctx, deps, org, "bluesky", mt, postTitle(p), p.URL, p.Text, p.Published) {
				saved++
			}
		}
	}
	for _, s := range org.Socials {
		if s.Network == "bluesky" && s.Handle != "" {
			run(bskyAPI+"app.bsky.feed.getAuthorFeed?limit=25&actor="+url.QueryEscape(strings.TrimPrefix(s.Handle, "@")), true)
		}
	}
	for _, q := range blueskyQueries(org) {
		run(bskyAPI+"app.bsky.feed.searchPosts?limit=25&sort=latest&q="+url.QueryEscape(q), false)
	}
	if saved > 0 {
		slog.Info("watchlist/bluesky: done", "org", org.Name, "new_hits", saved)
	}
	return saved
}
