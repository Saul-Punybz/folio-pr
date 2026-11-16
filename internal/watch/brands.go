package watch

import (
	"context"
	"log/slog"

	"github.com/Saul-Punybz/folio/internal/models"
)

// Plantillas de marca de la organización. Cada dato salió de una página
// pública revisada el 16 nov 2026 (la URL queda en SourcesEvidence). Lo que
// no tiene evidencia no se pone. Se siembran una vez; luego son de la usuaria.

const (
	evLISC = "https://www.lisc.org/our-resources/resource/oasis-hub/"
	evNFF  = "https://nff.org/insights/oasis-hub/"
	evPSI  = "https://impactpr.co/"
	evO360 = "https://impactpr.co/oasis360"
	evPM   = "https://www.projectmakerspr.org/"
	evVim  = "https://vimenti.org/"
	evNIMB = "https://newsismybusiness.com/project-makers-promotes-community-based-businesses-in-san-juan"
)

// BrandTemplates devuelve las 4 marcas sembradas.
func BrandTemplates() []models.WatchlistOrg {
	return []models.WatchlistOrg{
		{
			Name:     "Project Makers",
			Kind:     models.KindMarca,
			Website:  "https://www.projectmakerspr.org/",
			Aliases:  []string{"Project Makers PR", "Project Makers", "ProjectMakersPR", "projectmakerspr"},
			Hashtags: []string{"historiadeexito", "projectmakerspr"},
			Socials: []models.Social{
				{Network: "instagram", Handle: "projectmakerspr", URL: "https://www.instagram.com/projectmakerspr/"},
				{Network: "facebook", Handle: "ProjectMakersPR", URL: "https://www.facebook.com/ProjectMakersPR"},
				{Network: "linkedin", Handle: "project-makers-pr", URL: "https://www.linkedin.com/company/project-makers-pr/"},
				{Network: "youtube", Handle: "UCVKHr_P6fayHzePCCi-JCTQ", URL: "https://www.youtube.com/@ProjectMakersPR"},
			},
			SourcesEvidence: []models.Evidence{
				{Dato: "sitio oficial (confirmado por el dueño)", URL: evPM},
				{Dato: "Instagram y LinkedIn: dado por el dueño, 16 nov 2026", URL: "https://www.instagram.com/projectmakerspr/"},
				{Dato: "LinkedIn: dado por el dueño, 16 nov 2026", URL: "https://www.linkedin.com/company/project-makers-pr/"},
				{Dato: "Facebook ProjectMakersPR (pie del sitio; el pie también enlaza instagram.com/projctmakerspr, con errata)", URL: evPM},
				{Dato: "RSS del blog", URL: "https://www.projectmakerspr.org/blog-feed.xml"},
				{Dato: "YouTube (la página del canal enlaza projectmakerspr.org)", URL: "https://www.youtube.com/channel/UCVKHr_P6fayHzePCCi-JCTQ/about"},
				{Dato: "hashtag #historiadeexito (títulos de sus videos de YouTube)", URL: "https://www.youtube.com/feeds/videos.xml?channel_id=UCVKHr_P6fayHzePCCi-JCTQ"},
				{Dato: "#projectmakerspr: hashtag propuesto (derivado del handle); sin uso visto en Bluesky, Mastodon, YouTube ni en el sitio el 16 nov 2026", URL: "https://www.instagram.com/projectmakerspr/"},
				{Dato: "brazo empresarial de Oasis 360, apoyado por PSI", URL: evNIMB},
				{Dato: "opera el área de negocios del Oasis Hub", URL: evO360},
			},
		},
		{
			Name:         "Vimenti",
			Kind:         models.KindMarca,
			Website:      "https://vimenti.org/",
			Aliases:      []string{"Vimenti", "Vimenti School", "Escuela Vimenti", "Vimenti Charter School", "vimentipr"},
			ExcludeTerms: []string{"vimentin", "princess-vimenti", "princess vimenti"},
			Hashtags:     []string{"Vimenti", "VimentiLaFamiliaEsElCentro", "ModeloDeDosGeneraciones", "expansion2gen", "vimentipr"},
			Socials: []models.Social{
				{Network: "instagram", Handle: "vimentipr", URL: "https://www.instagram.com/vimentipr/"},
				{Network: "facebook", Handle: "VimentiPR", URL: "https://www.facebook.com/VimentiPR"},
				{Network: "x", Handle: "vimenti", URL: "https://twitter.com/vimenti"},
				{Network: "linkedin", Handle: "vimentipr", URL: "https://www.linkedin.com/company/vimentipr/"},
				{Network: "youtube", Handle: "UCtIP11PWtx-QX_odLsQVLbw", URL: "https://www.youtube.com/@VimentiPR"},
			},
			SourcesEvidence: []models.Evidence{
				{Dato: "sitio oficial (confirmado por el dueño)", URL: evVim},
				{Dato: "Instagram, Facebook y LinkedIn: dado por el dueño, 16 nov 2026", URL: "https://www.facebook.com/VimentiPR"},
				{Dato: "X @vimenti (cabecera y pie del sitio; el sitio enlaza además linkedin.com/company/vimentipuertorico y un Facebook viejo)", URL: evVim},
				{Dato: "RSS del sitio", URL: "https://vimenti.org/feed/"},
				{Dato: "YouTube (la página del canal enlaza vimenti.org)", URL: "https://www.youtube.com/channel/UCtIP11PWtx-QX_odLsQVLbw/about"},
				{Dato: "hashtags (títulos de sus videos de YouTube)", URL: "https://www.youtube.com/feeds/videos.xml?channel_id=UCtIP11PWtx-QX_odLsQVLbw"},
				{Dato: "Vimenti Charter School en el Oasis Hub", URL: evLISC},
				{Dato: "#vimentipr: hashtag propuesto (derivado del handle); sin uso visto en Bluesky, Mastodon, YouTube ni en el sitio el 16 nov 2026", URL: "https://www.instagram.com/vimentipr/"},
			},
		},
		{
			Name:           "Oasis 360",
			Kind:           models.KindMarca,
			Website:        "https://impactpr.co/oasis360.html",
			Aliases:        []string{"Oasis 360", "Oasis360", "oasis360_pr"},
			ContextAliases: []string{"Oasis Hub", "Proyecto Oasis", "Oasis Expansion"},
			Hashtags:       []string{"oasis360", "oasis360pr"},
			ExcludeTerms: []string{
				"Gallagher", "Wonderwall", "Britpop", "banda británica", "Oasis Live", "reunion tour",
				"Oasis of the Seas", "Royal Caribbean", "hotel", "resort", "Cordillerano",
				"Benahavís", "Bath", "Waterloo", "Townsville", "Oasis360 Solutions", "Bizapedia", "Oasis Hub LLC",
			},
			Socials: []models.Social{
				{Network: "instagram", Handle: "oasis360_pr", URL: "https://www.instagram.com/oasis360_pr/"},
				{Network: "facebook", Handle: "Oasis 360", URL: "https://www.facebook.com/profile.php?id=61580984582881"},
			},
			SourcesEvidence: []models.Evidence{
				{Dato: "Instagram: dado por el dueño, 16 nov 2026", URL: "https://www.instagram.com/oasis360_pr/"},
				{Dato: "Facebook: dado por el dueño, 16 nov 2026; su título público es «Oasis 360»", URL: "https://www.facebook.com/profile.php?id=61580984582881"},
				{Dato: "fuente dada por el dueño: Oasis Hub (LISC, préstamo puente de $3.71M)", URL: evLISC},
				{Dato: "fuente dada por el dueño: Oasis 360 y el Oasis Hub (PSI)", URL: evO360},
				{Dato: "fuente dada por el dueño: OASIS Hub (Nonprofit Finance Fund)", URL: evNFF},
				{Dato: "#oasis360 y #oasis360pr: hashtag propuesto (derivado del handle); sin uso visto en Bluesky, Mastodon, YouTube ni en el sitio el 16 nov 2026", URL: "https://www.instagram.com/oasis360_pr/"},
				{Dato: "alias «Proyecto Oasis» (prensa)", URL: "https://newsismybusiness.com/100m-socio-educational-initiative-proyecto-oasis-breaks-ground-in-san-juan"},
			},
		},
		{
			Name:           "Platform for Social Impact (PSI)",
			Kind:           models.KindMarca,
			Website:        "https://impactpr.co/",
			Aliases:        []string{"impactpr.co", "socialimpact.pr"},
			ContextAliases: []string{"Platform for Social Impact", "Platform for Social Change", "Impact PR", "PSI"},
			Hashtags:       []string{"socialimpactpr", "platformforsocialimpact"},
			People: []models.Person{
				{Name: "Eduardo Carrera", Role: "Founder & CEO", URL: "https://www.linkedin.com/in/eduardo-carrera-psi/"},
			},
			ExcludeTerms: []string{
				"pounds per square inch", "libras por pulgada", "Population Services International",
				"PSI Services", "public relations", "relaciones públicas", "PR agency", "PR firm",
				"Pacific Symposium", "Psi Chi", "Phi Chi Psi", "Chi Psi",
			},
			Socials: []models.Social{
				{Network: "instagram", Handle: "socialimpact.pr", URL: "https://www.instagram.com/socialimpact.pr/"},
				{Network: "facebook", Handle: "Platform for Social Impact", URL: "https://www.facebook.com/profile.php?id=61562225174230"},
				{Network: "linkedin", Handle: "platform-for-social-impact", URL: "https://www.linkedin.com/company/platform-for-social-impact/"},
			},
			SourcesEvidence: []models.Evidence{
				{Dato: "sitio oficial Impact PR (confirmado por el dueño)", URL: evPSI},
				{Dato: "Instagram y LinkedIn: dado por el dueño, 16 nov 2026", URL: "https://www.linkedin.com/company/platform-for-social-impact/"},
				{Dato: "Facebook: dado por el dueño, 16 nov 2026; su título público es «Platform for Social Impact»", URL: "https://www.facebook.com/profile.php?id=61562225174230"},
				{Dato: "Eduardo Carrera, «PSI Founder & CEO»", URL: "https://impactpr.co/who-we-are.html"},
				{Dato: "perfil de LinkedIn de Eduardo Carrera: dado por el dueño, 16 nov 2026", URL: "https://www.linkedin.com/in/eduardo-carrera-psi/"},
				{Dato: "el sitio no enlaza redes propias (solo Vidalus y sus programas)", URL: evPSI},
				{Dato: "PSI, organización paraguas del Oasis Hub", URL: evLISC},
				{Dato: "#socialimpactpr y #platformforsocialimpact: hashtag propuesto (derivado del handle); sin uso visto en Bluesky, Mastodon, YouTube ni en el sitio el 16 nov 2026", URL: "https://www.instagram.com/socialimpact.pr/"},
				{Dato: "Oasis 360 es el sistema de PSI", URL: evO360},
			},
		},
	}
}

// SeedBrandTemplates siembra las plantillas (idempotente; sin admin no hace nada).
func SeedBrandTemplates(ctx context.Context, orgs *models.WatchlistOrgStore) {
	if orgs == nil {
		return
	}
	n, err := orgs.SeedTemplates(ctx, BrandTemplates())
	if err != nil {
		slog.Warn("marcas: sembrar plantilla", "err", err)
		return
	}
	if n > 0 {
		slog.Info("marcas: plantilla sembrada", "creadas", n)
	}
}
