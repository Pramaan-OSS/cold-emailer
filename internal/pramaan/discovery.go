package pramaan

func init() {
	publicReads["companies.connections.list"] = true
	publicReads["companies.discovery-count"] = true
	publicReads["companies.discovery-search"] = true
	publicReads["companies.domain.find"] = true
	publicReads["companies.domain.find-bulk"] = true
	publicReads["companies.enrich"] = true
	publicReads["companies.enrich-bulk"] = true
	publicReads["companies.footprint.growth"] = true
	publicReads["companies.footprint.score"] = true
	publicReads["companies.icp.validate-bulk"] = true
	publicReads["companies.lookalike.search"] = true
	publicReads["companies.match-search"] = true
	publicReads["companies.profile.lookup"] = true
	publicReads["companies.research-bulk"] = true
	publicReads["companies.search"] = true
	publicReads["companies.search-bulk"] = true
	publicReads["companies.search-filtered"] = true
	publicReads["companies.segment"] = true
	publicReads["companies.similar"] = true
	publicReads["people.company.search"] = true
	publicReads["people.email.find-bulk"] = true
	publicReads["people.enrich"] = true
	publicReads["people.enrich-bulk"] = true
	publicReads["people.search"] = true
	publicReads["people.search-count"] = true
	publicReads["people.search-export"] = true
	publicReads["web.domain.redirects"] = true
	publicReads["web.domains.related"] = true
	publicReads["web.read"] = true
}
