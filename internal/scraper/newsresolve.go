package scraper

import (
	"net/url"
	"strings"
)

// Fuentes de búsqueda (Google News y Bing News): un solo feed que reúne
// noticias de muchos medios. Para ellas la región de cada ítem sale del medio
// real, no de la fuente.

// isSearchAggregator dice si feedURL es un feed de búsqueda de Google News o
// de Bing News.
func isSearchAggregator(feedURL string) bool {
	u := strings.ToLower(feedURL)
	return strings.Contains(u, "news.google.com/rss") || strings.Contains(u, "bing.com/news/search")
}

// ResolveBingNewsURL devuelve el URL real de un enlace de Bing News
// (bing.com/news/apiclick.aspx?...&url=<real>). Si no es de Bing o no trae el
// parámetro, devuelve link sin cambios.
//
// Los enlaces de Google News (news.google.com/rss/articles/...) son opacos y
// resolverlos exige un endpoint interno no documentado de Google, que no se
// usa: para esos el medio real se conoce por el elemento <source> del feed.
func ResolveBingNewsURL(link string) string {
	u, err := url.Parse(link)
	if err != nil || !strings.HasSuffix(strings.ToLower(u.Hostname()), "bing.com") {
		return link
	}
	if real := u.Query().Get("url"); strings.HasPrefix(real, "http") {
		return real
	}
	return link
}

// hostOf devuelve el host en minúsculas y sin "www.".
func hostOf(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
}

func hostMatches(host string, domains []string) bool {
	for _, d := range domains {
		if host == d || strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	return false
}

// Medios de Puerto Rico que pueden no estar en la tabla de fuentes.
var prMediaDomains = []string{
	"elnuevodia.com", "primerahora.com", "elvocero.com", "noticel.com", "metro.pr",
	"telemundopr.com", "wapa.tv", "noticentro.tv", "newsismybusiness.com", "sincomillas.com",
	"80grados.net", "puertoricoreport.com", "theweeklyjournal.com", "caribbeanbusiness.com",
	"elcalce.com", "jayfonseca.com", "periodismoinvestigativo.com", "upr.edu", "wipr.pr",
	"radioisla.tv", "teleonce.com", "esnoticiapr.com", "prensacomunitaria.org", "elsurnews.com",
	"sanjuandaily.com", "fcpr.org", "foundationforpuertorico.org", "colmena66.com", "inprende.com",
	"prsciencetrust.org", "tucamarapr.org", "vimenti.org", "oasis360.org",
}

// Medios de Estados Unidos (los .gov, .edu, .mil y .us se reconocen aparte).
var usMediaDomains = []string{
	"nytimes.com", "washingtonpost.com", "wsj.com", "cnn.com", "cbsnews.com", "nbcnews.com",
	"abcnews.go.com", "foxnews.com", "apnews.com", "npr.org", "pbs.org", "usatoday.com",
	"latimes.com", "miamiherald.com", "politico.com", "axios.com", "thehill.com", "bloomberg.com",
	"forbes.com", "cnbc.com", "msnbc.com", "newsweek.com", "time.com", "vox.com", "huffpost.com",
	"theatlantic.com", "newyorker.com", "propublica.org", "nbcmiami.com", "wlrn.org", "univision.com",
	"telemundo.com", "telemundo51.com", "al.com", "orlandosentinel.com", "sun-sentinel.com",
	"tampabay.com", "bostonglobe.com", "chicagotribune.com", "nydailynews.com", "nypost.com",
	"philanthropy.com", "nonprofitquarterly.org", "techcrunch.com", "wired.com", "theverge.com",
	"businessinsider.com", "reuters.com", "buzzfeednews.com", "slate.com", "salon.com",
	"commondreams.org", "democracynow.org", "motherjones.com", "thenation.com", "newrepublic.com",
	"hispanicexecutive.com", "latinpost.com", "hispanicfederation.org", "weather.com", "accuweather.com",
}

// RegionForDomain clasifica un medio por su dominio: "PR" si es de Puerto
// Rico, "US" si es de Estados Unidos y "Global" en cualquier otro caso.
// extraPR son dominios de PR adicionales (p. ej. los de las fuentes activas).
func RegionForDomain(host string, extraPR []string) string {
	host = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(host)), "www.")
	switch {
	case host == "":
		return "Global"
	case strings.HasSuffix(host, ".pr") || strings.HasSuffix(host, ".pr.gov") || strings.HasSuffix(host, ".pr.us"),
		hostMatches(host, prMediaDomains), hostMatches(host, extraPR):
		return "PR"
	case strings.HasSuffix(host, ".gov") || strings.HasSuffix(host, ".edu") || strings.HasSuffix(host, ".mil") ||
		strings.HasSuffix(host, ".us"), hostMatches(host, usMediaDomains):
		return "US"
	}
	return "Global"
}
