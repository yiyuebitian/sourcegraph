package gitlab

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/pkg/errors"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/gitlab"
)

// FetchUserPerms returns a list of project IDs (on code host) that the given account
// has read access on the code host. The project ID has the same value as it would be
// used as api.ExternalRepoSpec.ID. The returned list only includes private project IDs.
//
// This method may return partial but valid results in case of error, and it is up to
// callers to decide whether to discard.
//
// API docs: https://docs.gitlab.com/ee/api/projects.html#list-all-projects
func (p *SudoProvider) FetchUserPerms(ctx context.Context, account *extsvc.ExternalAccount) ([]string, error) {
	if account == nil {
		return nil, errors.New("no account provided")
	} else if account.ServiceType != p.codeHost.ServiceType || account.ServiceID != p.codeHost.ServiceID {
		return nil, fmt.Errorf("service mismatch: want %q - %q but the account has %q - %q",
			p.codeHost.ServiceType, p.codeHost.ServiceID, account.ServiceType, account.ServiceID)
	}

	user, _, err := gitlab.GetExternalAccountData(&account.ExternalAccountData)
	if err != nil {
		return nil, errors.Wrap(err, "get external account data")
	}
	sudo := strconv.Itoa(int(user.ID))

	q := make(url.Values)
	q.Add("visibility", "private")  // This method is meant to return only private projects
	q.Add("min_access_level", "20") // 20 => Reporter access (i.e. have access to project code)
	q.Add("per_page", "100")        // 100 is the maximum page size

	// The next URL to request for projects, and it is reused in the succeeding for loop.
	nextURL := "projects?" + q.Encode()

	// 100 matches the maximum page size, thus a good default to avoid multiple allocations
	// when appending the first 100 results to the slice.
	projectIDs := make([]string, 0, 100)

	client := p.clientProvider.GetPATClient(p.sudoToken, sudo)
	for {
		projects, next, err := client.ListProjects(ctx, nextURL)
		if err != nil {
			return projectIDs, err
		}

		for i := range projects {
			projectIDs = append(projectIDs, strconv.Itoa(projects[i].ID))
		}

		if next == nil {
			break
		}
		nextURL = *next
	}

	return projectIDs, nil
}

// FetchRepoPerms returns a list of user IDs (on code host) who have read ccess to
// the given project on the code host. The user ID has the same value as it would
// be used as extsvc.ExternalAccount.AccountID. The returned list includes both
// direct access and inherited from the group membership.
//
// API docs: https://docs.gitlab.com/ee/api/members.html#list-all-members-of-a-group-or-project-including-inherited-members
func (p *SudoProvider) FetchRepoPerms(ctx context.Context, repo *api.ExternalRepoSpec) ([]string, error) {
	if repo == nil {
		return nil, errors.New("no repository provided")
	} else if repo.ServiceType != p.codeHost.ServiceType || repo.ServiceID != p.codeHost.ServiceID {
		return nil, fmt.Errorf("service mismatch: want %q - %q but the repository has %q - %q",
			p.codeHost.ServiceType, p.codeHost.ServiceID, repo.ServiceType, repo.ServiceID)
	}

	q := make(url.Values)
	q.Add("per_page", "100") // 100 is the maximum page size

	// The next URL to request for members, and it is reused in the succeeding for loop.
	nextURL := fmt.Sprintf("projects/%s/members/all?%s", repo.ID, q.Encode())

	// 100 matches the maximum page size, thus a good default to avoid multiple allocations
	// when appending the first 100 results to the slice.
	userIDs := make([]string, 0, 100)

	client := p.clientProvider.GetPATClient(p.sudoToken, "")
	for {
		members, next, err := client.ListMembers(ctx, nextURL)
		if err != nil {
			return userIDs, err
		}

		for i := range members {
			userIDs = append(userIDs, strconv.Itoa(int(members[i].ID)))
		}

		if next == nil {
			break
		}
		nextURL = *next
	}

	return userIDs, nil
}
