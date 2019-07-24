package graphqlbackend

import (
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/campaigns"
)

func init() {
	// Contribute the GraphQL type CampaignsMutation.
	graphqlbackend.Campaigns = campaigns.GraphQLResolver{}
}