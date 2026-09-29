package swh

import "github.com/andrew/swh-go/internal/gen"

// Response types, aliased from the generated package so callers need not
// import it. Each is named after the route that first returns its shape;
// several routes share one where the archive documents them identically.
type (
	Content           = gen.Content
	ContentFiletype   = gen.ContentFiletype
	ContentLanguage   = gen.ContentLanguage
	ContentLicense    = gen.ContentLicense
	Directory         = gen.Directory
	Origin            = gen.OriginGet
	OriginSearchable  = gen.OriginSearch
	OriginVisit       = gen.OriginVisits
	OriginVisitLatest = gen.OriginVisitLatest
	Release           = gen.Release
	Revision          = gen.Revision
	Snapshot          = gen.Snapshot
	StatCounters      = gen.StatCounters
)

// Query parameter types for the endpoints that take them.
type (
	Api1OriginSearchParams          = gen.Api1OriginSearchParams
	Api1OriginVisitLatestParams     = gen.Api1OriginVisitLatestParams
	Api1VaultCookGitBareSwhidParams = gen.Api1VaultCookGitBareSwhidParams
	Api1OriginVisitsParams          = gen.Api1OriginVisitsParams
	Api1SnapshotParams              = gen.Api1SnapshotParams
)
